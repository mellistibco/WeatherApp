package flowinst

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"sync"

	"github.com/TIBCOSoftware/flogo-lib/core/data"
	"github.com/TIBCOSoftware/flogo-lib/flow/activity"
	"github.com/TIBCOSoftware/flogo-lib/flow/flowdef"
	"github.com/TIBCOSoftware/flogo-lib/flow/model"
	"github.com/TIBCOSoftware/flogo-lib/flow/support"
	"github.com/TIBCOSoftware/flogo-lib/logger"
	"github.com/TIBCOSoftware/flogo-lib/util"
)

const (
	idEhTasEnv    = 0
	idRootTaskEnv = 1
)

// Instance is a structure for representing an instance of a Flow
type Instance struct {
	id          string
	stepID      int
	lock        sync.Mutex
	status      Status
	state       int
	FlowURI     string
	Flow        *flowdef.Definition
	RootTaskEnv *TaskEnv
	EhTaskEnv   *TaskEnv
	FlowModel   *model.FlowModel
	Attrs       map[string]*data.Attribute
	Patch       *support.Patch
	Interceptor *support.Interceptor

	WorkItemQueue *util.SyncQueue //todo: change to faster non-threadsafe queue

	wiCounter     int
	ChangeTracker *InstanceChangeTracker `json:"-"`

	flowProvider flowdef.Provider
	replyHandler support.ReplyHandler
}

// New creates a new Flow Instance from the specified Flow
func New(instanceID string, flowURI string, flow *flowdef.Definition, flowModel *model.FlowModel) *Instance {
	var instance Instance
	instance.id = instanceID
	instance.stepID = 0
	instance.FlowURI = flowURI
	instance.Flow = flow
	instance.FlowModel = flowModel
	instance.status = StatusNotStarted
	instance.WorkItemQueue = util.NewSyncQueue()
	instance.ChangeTracker = NewInstanceChangeTracker()

	var taskEnv TaskEnv
	taskEnv.ID = idRootTaskEnv
	taskEnv.Task = flow.RootTask()
	taskEnv.taskID = flow.RootTask().ID()
	taskEnv.Instance = &instance
	taskEnv.TaskDatas = make(map[int]*TaskData)
	taskEnv.LinkDatas = make(map[int]*LinkData)

	instance.RootTaskEnv = &taskEnv

	return &instance
}

// NewFlowInstance creates a new Flow Instance from the specified Flow
func NewFlowInstance(instanceID string, flowURI string, flow *flowdef.Definition) *Instance {

	var instance Instance
	instance.id = instanceID
	instance.stepID = 0
	instance.FlowURI = flowURI
	instance.Flow = flow
	instance.FlowModel = model.Get(flow.ModelID())
	instance.status = StatusNotStarted
	instance.WorkItemQueue = util.NewSyncQueue()
	instance.ChangeTracker = NewInstanceChangeTracker()

	var taskEnv TaskEnv
	taskEnv.ID = idRootTaskEnv
	taskEnv.Task = flow.RootTask()
	taskEnv.taskID = flow.RootTask().ID()
	taskEnv.Instance = &instance
	taskEnv.TaskDatas = make(map[int]*TaskData)
	taskEnv.LinkDatas = make(map[int]*LinkData)

	instance.RootTaskEnv = &taskEnv

	return &instance
}

// SetFlowProvider sets the process.Provider that the instance should use
func (pi *Instance) SetFlowProvider(provider flowdef.Provider) {
	pi.flowProvider = provider
}

// Restart indicates that this FlowInstance was restarted
func (pi *Instance) Restart(id string, provider flowdef.Provider) {
	pi.id = id
	pi.flowProvider = provider
	pi.Flow, _ = pi.flowProvider.GetFlow(pi.FlowURI)
	pi.FlowModel = model.Get(pi.Flow.ModelID())
	pi.RootTaskEnv.init(pi)
}

// ID returns the ID of the Flow Instance
func (pi *Instance) ID() string {
	return pi.id
}

// Name implements activity.FlowDetails.Name method
func (pi *Instance) Name() string {
	return pi.Flow.Name()
}

// ReplyHandler returns the reply handler for the instance
func (pi *Instance) ReplyHandler() support.ReplyHandler {
	return pi.replyHandler
}

// SetReplyHandler sets the reply handler for the instance
func (pi *Instance) SetReplyHandler(replyHandler support.ReplyHandler) {
	pi.replyHandler = replyHandler
}

// FlowDefinition returns the Flow that the instance is of
func (pi *Instance) FlowDefinition() *flowdef.Definition {
	return pi.Flow
}

// StepID returns the current step ID of the Flow Instance
func (pi *Instance) StepID() int {
	return pi.stepID
}

// Status returns the current status of the Flow Instance
func (pi *Instance) Status() Status {
	return pi.status
}

func (pi *Instance) setStatus(status Status) {

	pi.status = status
	pi.ChangeTracker.SetStatus(status)
}

// State returns the state indicator of the Flow Instance
func (pi *Instance) State() int {
	return pi.state
}

// SetState sets the state indicator of the Flow Instance
func (pi *Instance) SetState(state int) {
	pi.state = state
	pi.ChangeTracker.SetState(state)
}

// UpdateAttrs updates the attributes of the Flow Instance
func (pi *Instance) UpdateAttrs(attrs []*data.Attribute) {

	if attrs != nil {

		logger.Debugf("Updating flow attrs: %v", attrs)

		if pi.Attrs == nil {
			pi.Attrs = make(map[string]*data.Attribute, len(attrs))
		}

		for _, attr := range attrs {
			pi.Attrs[attr.Name] = data.NewAttribute(attr.Name, attr.Type, attr.Value)
		}
	}
}

// Start will start the Flow Instance, returns a boolean indicating
// if it was able to start
func (pi *Instance) Start(startAttrs []*data.Attribute) bool {

	pi.setStatus(StatusActive)

	//apply inputMapper if we have one, otherwise do default mappings
	applyDefaultInstanceInputMappings(pi, startAttrs)

	logger.Infof("FlowInstance Flow: %v", pi.FlowModel)
	model := pi.FlowModel.GetFlowBehavior()

	//todo: error if model not found

	ok, evalCode := model.Start(pi)

	if ok {
		rootTaskData := pi.RootTaskEnv.NewTaskData(pi.Flow.RootTask())

		pi.scheduleEval(rootTaskData, evalCode)
	}

	return ok
}

////Resume resumes a Flow Instance
//func (pi *Instance) Resume(flowData map[string]interface{}) bool {
//
//	model := pi.FlowModel.GetFlowBehavior(pi.Flow.TypeID())
//
//	pi.setStatus(StatusActive)
//	pi.UpdateAttrs(flowData)
//
//	return model.Resume(pi)
//}

// DoStep performs a single execution 'step' of the Flow Instance
func (pi *Instance) DoStep() bool {

	hasNext := false

	pi.ResetChanges()

	pi.stepID++

	if pi.status == StatusActive {

		item, ok := pi.WorkItemQueue.Pop()

		if ok {
			logger.Debug("popped item off queue")

			workItem := item.(*WorkItem)

			pi.ChangeTracker.trackWorkItem(&WorkItemQueueChange{ChgType: CtDel, ID: workItem.ID, WorkItem: workItem})

			pi.execTask(workItem)
			hasNext = true
		} else {
			logger.Debug("queue emtpy")
		}
	}

	return hasNext
}

// GetChanges returns the Change Tracker object
func (pi *Instance) GetChanges() *InstanceChangeTracker {
	return pi.ChangeTracker
}

// ResetChanges resets an changes that were being tracked
func (pi *Instance) ResetChanges() {

	if pi.ChangeTracker != nil {
		pi.ChangeTracker.ResetChanges()
	}

	//todo: can we reuse this to avoid gc
	pi.ChangeTracker = NewInstanceChangeTracker()
}

func (pi *Instance) scheduleEval(taskData *TaskData, evalCode int) {

	pi.wiCounter++

	workItem := NewWorkItem(pi.wiCounter, taskData, EtEval, evalCode)
	logger.Debugf("Scheduling EVAL on task: %s\n", taskData.task.Name())

	pi.WorkItemQueue.Push(workItem)
	pi.ChangeTracker.trackWorkItem(&WorkItemQueueChange{ChgType: CtAdd, ID: workItem.ID, WorkItem: workItem})
}

// execTask executes the specified Work Item of the Flow Instance
func (pi *Instance) execTask(workItem *WorkItem) {

	defer func() {
		if r := recover(); r != nil {

			err := fmt.Errorf("Unhandled Error executing task '%s' : %v\n", workItem.TaskData.task.Name(), r)
			logger.Error(err)

			// todo: useful for debugging
			logger.Debugf("StackTrace: %s", debug.Stack())

			pi.handleError(workItem.TaskData, activity.NewError(err.Error()))
		}
	}()

	taskData := workItem.TaskData
	taskBehavior := pi.FlowModel.GetTaskBehavior(taskData.task.TypeID())

	var done bool
	var doneCode int
	var err error

	//todo: should validate process activities

	if workItem.ExecType == EtEval {

		eval := true

		if taskData.HasAttrs() {

			applyInputMapper(pi, taskData)
			eval = applyInputInterceptor(pi, taskData)
		}

		if eval {
			done, doneCode, err = taskBehavior.Eval(taskData, workItem.EvalCode)
		} else {
			done = true
		}
	} else {
		done, doneCode, err = taskBehavior.PostEval(taskData, workItem.EvalCode, nil)
	}

	if err != nil {
		pi.handleError(taskData, err)
		return
	}

	if done {

		if taskData.HasAttrs() {
			applyOutputInterceptor(pi, taskData)

			appliedMapper := applyOutputMapper(pi, taskData)

			if !appliedMapper && !taskData.task.IsScope() {

				logger.Debug("Applying Default Output Mapping")
				applyDefaultActivityOutputMappings(pi, taskData)
			}
		}

		pi.handleTaskDone(taskBehavior, taskData, doneCode)
	}
}

func (pi *Instance) handleError(taskData *TaskData, err error) {

	pi.AddAttr("{E.activity}", data.STRING, taskData.TaskName())
	pi.AddAttr("{E.message}", data.STRING, err.Error())

	if aerr, ok := err.(*activity.Error); ok {
		pi.AddAttr("{E.data}", data.OBJECT, aerr.Data())
	}

	if taskData.taskEnv.ID != idEhTasEnv {
		pi.HandleError()
	}
}

// handleTaskDone handles the completion of a task in the Flow Instance
func (pi *Instance) handleTaskDone(taskBehavior model.TaskBehavior, taskData *TaskData, doneCode int) {

	notifyParent, childDoneCode, taskEntries := taskBehavior.Done(taskData, doneCode)

	task := taskData.Task()

	if notifyParent {

		parentTask := task.Parent()

		if parentTask != nil {
			parentTaskData := taskData.taskEnv.TaskDatas[parentTask.ID()]
			parentBehavior := pi.FlowModel.GetTaskBehavior(parentTask.TypeID())
			parentDone, parentDoneCode := parentBehavior.ChildDone(parentTaskData, task, childDoneCode)

			if parentDone {
				pi.handleTaskDone(parentBehavior, parentTaskData, parentDoneCode)
			}

		} else {

			//todo distinguish between error handler env and rootTaskEnv

			//Root Task is Done, so notify Flow
			flowBehavior := pi.FlowModel.GetFlowBehavior()
			flowBehavior.TasksDone(pi, childDoneCode)
			flowBehavior.Done(pi)

			pi.setStatus(StatusCompleted)
		}
	}

	if len(taskEntries) > 0 {

		for _, taskEntry := range taskEntries {

			logger.Debugf("execTask - TaskEntry: %v\n", taskEntry)
			taskToEnterBehavior := pi.FlowModel.GetTaskBehavior(taskEntry.Task.TypeID())

			enterTaskData, _ := taskData.taskEnv.FindOrCreateTaskData(taskEntry.Task)

			eval, evalCode := taskToEnterBehavior.Enter(enterTaskData, taskEntry.EnterCode)

			if eval {
				pi.scheduleEval(enterTaskData, evalCode)
			}
		}
	}

	taskData.taskEnv.releaseTask(task)
}

// HandleError handles instance errors
func (pi *Instance) HandleError() {

	if pi.Flow.ErrorHandlerTask() != nil {

		ehTask := pi.Flow.ErrorHandlerTask()

		if pi.EhTaskEnv == nil {
			var taskEnv TaskEnv
			taskEnv.ID = idEhTasEnv
			taskEnv.Task = ehTask
			taskEnv.taskID = ehTask.ID()
			taskEnv.Instance = pi
			taskEnv.TaskDatas = make(map[int]*TaskData)
			taskEnv.LinkDatas = make(map[int]*LinkData)

			pi.EhTaskEnv = &taskEnv
		}

		ehTaskData := pi.EhTaskEnv.TaskDatas[ehTask.ID()]

		if ehTaskData == nil {
			ehTaskData = pi.EhTaskEnv.NewTaskData(ehTask)
		}

		//todo: should we clear out the existing workitem queue?

		pi.scheduleEval(ehTaskData, 0)
	} else {

		//todo: log error information
		pi.setStatus(StatusFailed)
	}
}

// GetAttr implements data.Scope.GetAttr
func (pi *Instance) GetAttr(attrName string) (value *data.Attribute, exists bool) {

	if pi.Attrs != nil {
		attr, found := pi.Attrs[attrName]

		if found {
			return attr, true
		}
	}

	return pi.Flow.GetAttr(attrName)
}

// SetAttrValue implements api.Scope.SetAttrValue
func (pi *Instance) SetAttrValue(attrName string, value interface{}) error {
	if pi.Attrs == nil {
		pi.Attrs = make(map[string]*data.Attribute)
	}

	logger.Debugf("SetAttr - name: %s, value:%v\n", attrName, value)

	existingAttr, exists := pi.GetAttr(attrName)

	//todo: optimize, use existing attr
	if exists {
		attr := data.NewAttribute(attrName, existingAttr.Type, value)
		pi.Attrs[attrName] = attr
		pi.ChangeTracker.AttrChange(CtUpd, attr)
		return nil
	}

	return fmt.Errorf("Attr [%s] does not exists", attrName)
}

// AddAttr add a new attribute to the instance
func (pi *Instance) AddAttr(attrName string, attrType data.Type, value interface{}) *data.Attribute {
	if pi.Attrs == nil {
		pi.Attrs = make(map[string]*data.Attribute)
	}

	logger.Debugf("AddAttr - name: %s, type: %s, value:%v\n", attrName, attrType, value)

	var attr *data.Attribute

	existingAttr, exists := pi.GetAttr(attrName)

	if exists {
		attr = existingAttr
	} else {
		attr = data.NewAttribute(attrName, attrType, value)
		pi.Attrs[attrName] = attr
		pi.ChangeTracker.AttrChange(CtAdd, attr)
	}

	return attr
}

////////////////////////////////////////////////////////////////////////////////////////////////////////
// Task Environment

// TaskEnv is a structure that describes the execution environment for a set of tasks
type TaskEnv struct {
	ID        int
	Task      *flowdef.Task
	Instance  *Instance
	ParentEnv *TaskEnv

	TaskDatas map[int]*TaskData
	LinkDatas map[int]*LinkData

	taskID int // for deserialization
}

// init initializes the Task Environment, typically called on deserialization
func (te *TaskEnv) init(flowInst *Instance) {

	if te.Instance == nil {

		te.Instance = flowInst
		te.Task = flowInst.Flow.GetTask(te.taskID)

		for _, v := range te.TaskDatas {
			v.taskEnv = te
			v.task = flowInst.Flow.GetTask(v.taskID)
		}

		for _, v := range te.LinkDatas {
			v.taskEnv = te
			v.link = flowInst.Flow.GetLink(v.linkID)
		}
	}
}

// FindOrCreateTaskData finds an existing TaskData or creates ones if not found for the
// specified task the task environment
func (te *TaskEnv) FindOrCreateTaskData(task *flowdef.Task) (taskData *TaskData, created bool) {

	taskData, ok := te.TaskDatas[task.ID()]

	created = false

	if !ok {
		taskData = NewTaskData(te, task)
		te.TaskDatas[task.ID()] = taskData
		te.Instance.ChangeTracker.trackTaskData(&TaskDataChange{ChgType: CtAdd, ID: task.ID(), TaskData: taskData})

		created = true
	}

	return taskData, created
}

// NewTaskData creates a new TaskData object
func (te *TaskEnv) NewTaskData(task *flowdef.Task) *TaskData {

	taskData := NewTaskData(te, task)
	te.TaskDatas[task.ID()] = taskData
	te.Instance.ChangeTracker.trackTaskData(&TaskDataChange{ChgType: CtAdd, ID: task.ID(), TaskData: taskData})

	return taskData
}

// FindOrCreateLinkData finds an existing LinkData or creates ones if not found for the
// specified link the task environment
func (te *TaskEnv) FindOrCreateLinkData(link *flowdef.Link) (linkData *LinkData, created bool) {

	linkData, ok := te.LinkDatas[link.ID()]
	created = false

	if !ok {
		linkData = NewLinkData(te, link)
		te.LinkDatas[link.ID()] = linkData
		te.Instance.ChangeTracker.trackLinkData(&LinkDataChange{ChgType: CtAdd, ID: link.ID(), LinkData: linkData})
		created = true
	}

	return linkData, created
}

// releaseTask cleans up TaskData in the task environment any of its dependencies.
// This is called when a task is completed and can be discarded
func (te *TaskEnv) releaseTask(task *flowdef.Task) {
	delete(te.TaskDatas, task.ID())
	te.Instance.ChangeTracker.trackTaskData(&TaskDataChange{ChgType: CtDel, ID: task.ID()})

	childTasks := task.ChildTasks()

	if len(childTasks) > 0 {

		for _, childTask := range childTasks {
			delete(te.TaskDatas, childTask.ID())
			te.Instance.ChangeTracker.trackTaskData(&TaskDataChange{ChgType: CtDel, ID: childTask.ID()})
		}
	}

	links := task.FromLinks()

	for _, link := range links {
		delete(te.LinkDatas, link.ID())
		te.Instance.ChangeTracker.trackLinkData(&LinkDataChange{ChgType: CtDel, ID: link.ID()})
	}
}

// TaskData represents data associated with an instance of a Task
type TaskData struct {
	taskEnv *TaskEnv
	task    *flowdef.Task
	state   int
	done    bool
	attrs   map[string]*data.Attribute

	inScope  data.Scope
	outScope data.Scope

	changes int

	taskID int //needed for serialization
}

// NewTaskData creates a TaskData for the specified task in the specified task
// environment
func NewTaskData(taskEnv *TaskEnv, task *flowdef.Task) *TaskData {
	var taskData TaskData

	taskData.taskEnv = taskEnv
	taskData.task = task

	//taskData.TaskID = task.ID

	return &taskData
}

// HasAttrs indicates if the task has attributes
func (td *TaskData) HasAttrs() bool {
	return len(td.task.ActivityType()) > 0 || td.task.IsScope()
}

/////////////////////////////////////////
// TaskData - TaskContext Implementation

// State implements flow.TaskContext.GetState
func (td *TaskData) State() int {
	return td.state
}

// SetState implements flow.TaskContext.SetState
func (td *TaskData) SetState(state int) {
	td.state = state
	td.taskEnv.Instance.ChangeTracker.trackTaskData(&TaskDataChange{ChgType: CtUpd, ID: td.task.ID(), TaskData: td})
}

// Task implements model.TaskContext.Task, by returning the Task associated with this
// TaskData object
func (td *TaskData) Task() *flowdef.Task {
	return td.task
}

// FromInstLinks implements model.TaskContext.FromInstLinks
func (td *TaskData) FromInstLinks() []model.LinkInst {

	logger.Debugf("FromInstLinks: task=%v\n", td.Task)

	links := td.task.FromLinks()

	numLinks := len(links)

	if numLinks > 0 {
		linkCtxs := make([]model.LinkInst, numLinks)

		for i, link := range links {
			linkCtxs[i], _ = td.taskEnv.FindOrCreateLinkData(link)
		}
		return linkCtxs
	}

	return nil
}

// ToInstLinks implements model.TaskContext.ToInstLinks,
func (td *TaskData) ToInstLinks() []model.LinkInst {

	logger.Debugf("ToInstLinks: task=%v\n", td.Task)

	links := td.task.ToLinks()

	numLinks := len(links)

	if numLinks > 0 {
		linkCtxs := make([]model.LinkInst, numLinks)

		for i, link := range links {
			linkCtxs[i], _ = td.taskEnv.FindOrCreateLinkData(link)
		}
		return linkCtxs
	}

	return nil
}

// ChildTaskInsts implements activity.ActivityContext.ChildTaskInsts method
func (td *TaskData) ChildTaskInsts() (taskInsts []model.TaskInst, hasChildTasks bool) {

	if len(td.task.ChildTasks()) == 0 {
		return nil, false
	}

	taskInsts = make([]model.TaskInst, 0)

	for _, task := range td.task.ChildTasks() {

		taskData, ok := td.taskEnv.TaskDatas[task.ID()]

		if ok {
			taskInsts = append(taskInsts, taskData)
		}
	}

	return taskInsts, true
}

// EnterLeadingChildren implements activity.ActivityContext.EnterLeadingChildren method
func (td *TaskData) EnterLeadingChildren(enterCode int) {

	//todo optimize
	for _, task := range td.task.ChildTasks() {

		if len(task.FromLinks()) == 0 {
			taskData, _ := td.taskEnv.FindOrCreateTaskData(task)
			taskBehavior := td.taskEnv.Instance.FlowModel.GetTaskBehavior(task.TypeID())

			eval, evalCode := taskBehavior.Enter(taskData, enterCode)

			if eval {
				td.taskEnv.Instance.scheduleEval(taskData, evalCode)
			}
		}
	}
}

// EnterChildren implements activity.ActivityContext.EnterChildren method
func (td *TaskData) EnterChildren(taskEntries []*model.TaskEntry) {

	if (taskEntries == nil) || (len(taskEntries) == 1 && taskEntries[0].Task == nil) {

		var enterCode int

		if taskEntries == nil {
			enterCode = 0
		} else {
			enterCode = taskEntries[0].EnterCode
		}

		logger.Debugf("Entering '%s' Task's %d children\n", td.task.Name(), len(td.task.ChildTasks()))

		for _, task := range td.task.ChildTasks() {

			taskData, _ := td.taskEnv.FindOrCreateTaskData(task)
			taskBehavior := td.taskEnv.Instance.FlowModel.GetTaskBehavior(task.TypeID())

			eval, evalCode := taskBehavior.Enter(taskData, enterCode)

			if eval {
				td.taskEnv.Instance.scheduleEval(taskData, evalCode)
			}
		}
	} else {

		for _, taskEntry := range taskEntries {

			//todo validate if specified task is child? or trust model

			taskData, _ := td.taskEnv.FindOrCreateTaskData(taskEntry.Task)
			taskBehavior := td.taskEnv.Instance.FlowModel.GetTaskBehavior(taskEntry.Task.TypeID())

			eval, evalCode := taskBehavior.Enter(taskData, taskEntry.EnterCode)

			if eval {
				td.taskEnv.Instance.scheduleEval(taskData, evalCode)
			}
		}
	}
}

// EvalLink implements activity.ActivityContext.EvalLink method
func (td *TaskData) EvalLink(link *flowdef.Link) (result bool, err error) {

	logger.Debugf("TaskContext.EvalLink: %d\n", link.ID())

	defer func() {
		if r := recover(); r != nil {
			logger.Warnf("Unhandled Error evaluating link '%s' : %v\n", link.ID(), r)

			// todo: useful for debugging
			logger.Debugf("StackTrace: %s", debug.Stack())

			if err != nil {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	mgr := td.taskEnv.Instance.Flow.GetLinkExprManager()

	if mgr != nil {
		result = mgr.EvalLinkExpr(link, td.taskEnv.Instance)
		return result, nil
	}

	return true, nil
}

// HasActivity implements activity.ActivityContext.HasActivity method
func (td *TaskData) HasActivity() bool {
	return activity.Get(td.task.ActivityType()) != nil
}

// EvalActivity implements activity.ActivityContext.EvalActivity method
func (td *TaskData) EvalActivity() (done bool, evalErr error) {

	act := activity.Get(td.task.ActivityType())

	//todo: if act == nil, return TaskDoesntHaveActivity error or something like that

	defer func() {
		if r := recover(); r != nil {
			logger.Warnf("Unhandled Error executing activity '%s'[%s] : %v\n", td.task.Name(), td.task.ActivityType(), r)

			// todo: useful for debugging
			logger.Debugf("StackTrace: %s", debug.Stack())

			if evalErr == nil {
				evalErr = activity.NewError(fmt.Sprintf("%v", r))
				done = false
			}
		}
	}()

	done, evalErr = act.Eval(td)

	return done, evalErr
}

// Failed marks the Activity as failed
func (td *TaskData) Failed(err error) {

	errorMsgAttr := "[A" + strconv.Itoa(td.task.ID()) + "._errorMsg]"
	td.taskEnv.Instance.AddAttr(errorMsgAttr, data.STRING, err.Error())
}

// FlowDetails implements activity.Context.FlowName method
func (td *TaskData) FlowDetails() activity.FlowDetails {
	return td.taskEnv.Instance
}

// TaskName implements activity.Context.TaskName method
func (td *TaskData) TaskName() string {
	return td.task.Name()
}

// InputScope get the InputScope of the task instance
func (td *TaskData) InputScope() data.Scope {

	if td.inScope != nil {
		return td.inScope
	}

	if len(td.task.ActivityType()) > 0 {

		act := activity.Get(td.task.ActivityType())
		td.inScope = NewFixedTaskScope(act.Metadata().Inputs, td.task)

	} else if td.task.IsScope() {

		//add flow scope
	}

	return td.inScope
}

// OutputScope get the InputScope of the task instance
func (td *TaskData) OutputScope() data.Scope {

	if td.outScope != nil {
		return td.outScope
	}

	if len(td.task.ActivityType()) > 0 {

		act := activity.Get(td.task.ActivityType())
		td.outScope = NewFixedTaskScope(act.Metadata().Outputs, nil)

		logger.Debugf("OutputScope: %v\n", td.outScope)
	} else if td.task.IsScope() {

		//add flow scope
	}

	return td.outScope
}

// GetInput implements activity.Context.GetInput
func (td *TaskData) GetInput(name string) interface{} {

	val, found := td.InputScope().GetAttr(name)
	if found {
		return val.Value
	}

	return nil
}

// SetOutput implements activity.Context.SetOutput
func (td *TaskData) SetOutput(name string, value interface{}) {

	logger.Debugf("SET OUTPUT: %s = %v\n", name, value)
	td.OutputScope().SetAttrValue(name, value)
}

// LinkData represents data associated with an instance of a Link
type LinkData struct {
	taskEnv *TaskEnv
	link    *flowdef.Link
	state   int

	changes int

	linkID int //needed for serialization
}

// NewLinkData creates a LinkData for the specified link in the specified task
// environment
func NewLinkData(taskEnv *TaskEnv, link *flowdef.Link) *LinkData {
	var linkData LinkData

	linkData.taskEnv = taskEnv
	linkData.link = link

	return &linkData
}

// State returns the current state indicator for the LinkData
func (ld *LinkData) State() int {
	return ld.state
}

// SetState sets the current state indicator for the LinkData
func (ld *LinkData) SetState(state int) {
	ld.state = state
	ld.taskEnv.Instance.ChangeTracker.trackLinkData(&LinkDataChange{ChgType: CtUpd, ID: ld.link.ID(), LinkData: ld})
}

// Link returns the Link associated with ld context
func (ld *LinkData) Link() *flowdef.Link {
	return ld.link
}

// ExecType is the type of execution to perform
type ExecType int

const (
	// EtEval denoted the Eval execution type
	EtEval ExecType = 10

	// EtPostEval denoted the PostEval execution type
	EtPostEval ExecType = 20
)

// WorkItem describes an item of work (event for a Task) that should be executed on Step
type WorkItem struct {
	ID       int       `json:"id"`
	TaskData *TaskData `json:"-"`
	ExecType ExecType  `json:"execType"`
	EvalCode int       `json:"code"`

	TaskID int `json:"taskID"` //for now need for ser
	//taskCtxID int `json:"taskCtxID"` //not needed for now
}

// NewWorkItem constructs a new WorkItem for the specified TaskData
func NewWorkItem(id int, taskData *TaskData, execType ExecType, evalCode int) *WorkItem {

	var workItem WorkItem

	workItem.ID = id
	workItem.TaskData = taskData
	workItem.ExecType = execType
	workItem.EvalCode = evalCode

	workItem.TaskID = taskData.task.ID()

	return &workItem
}
