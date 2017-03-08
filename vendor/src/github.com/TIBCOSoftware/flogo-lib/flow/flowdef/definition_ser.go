package flowdef

import (
	"github.com/TIBCOSoftware/flogo-lib/core/data"
	"github.com/TIBCOSoftware/flogo-lib/util"
)

// DefinitionRep is a serializable representation of a flow Definition
type DefinitionRep struct {
	ExplicitReply    bool              `json:"explicitReply"`
	Name             string            `json:"name"`
	ModelID          string            `json:"model"`
	Attributes       []*data.Attribute `json:"attributes,omitempty"`
	InputMappings    []*data.Mapping   `json:"inputMappings,omitempty"`
	RootTask         *TaskRep          `json:"rootTask"`
	ErrorHandlerTask *TaskRep          `json:"errorHandlerTask"`
}

// TaskRep is a serializable representation of a flow Task
type TaskRep struct {
	ID             int               `json:"id"`
	TypeID         int               `json:"type"`
	ActivityType   string            `json:"activityType"`
	Name           string            `json:"name"`
	Attributes     []*data.Attribute `json:"attributes,omitempty"`
	InputMappings  []*data.Mapping   `json:"inputMappings,omitempty"`
	OutputMappings []*data.Mapping   `json:"ouputMappings,omitempty"`
	Tasks          []*TaskRep        `json:"tasks,omitempty"`
	Links          []*LinkRep        `json:"links,omitempty"`
}

// LinkRep is a serializable representation of a flow Link
type LinkRep struct {
	ID     int    `json:"id"`
	Type   int    `json:"type"`
	Name   string `json:"name"`
	ToID   int    `json:"to"`
	FromID int    `json:"from"`
	Value  string `json:"value"`
}

// NewDefinition creates a flow Definition from a serializable
// definition representation
func NewDefinition(rep *DefinitionRep) (def *Definition, err error) {

	defer util.HandlePanic("NewDefinition", &err)

	def = &Definition{}
	def.name = rep.Name
	def.modelID = rep.ModelID
	def.explicitReply = rep.ExplicitReply

	if rep.InputMappings != nil {
		def.inputMapper = data.NewMapper(rep.InputMappings)
	}

	if len(rep.Attributes) > 0 {
		def.attrs = make(map[string]*data.Attribute, len(rep.Attributes))

		for _, value := range rep.Attributes {
			def.attrs[value.Name] = value
		}
	}

	def.rootTask = &Task{}

	def.tasks = make(map[int]*Task)
	def.links = make(map[int]*Link)

	addTask(def, def.rootTask, rep.RootTask)
	addLinks(def, def.rootTask, rep.RootTask)

	if rep.ErrorHandlerTask != nil {
		def.ehTask = &Task{}

		addTask(def, def.ehTask, rep.ErrorHandlerTask)
		addLinks(def, def.ehTask, rep.ErrorHandlerTask)
	}

	return def, nil
}

func addTask(def *Definition, task *Task, rep *TaskRep) {

	task.id = rep.ID
	task.activityType = rep.ActivityType
	task.typeID = rep.TypeID
	task.name = rep.Name
	//task.Definition = def

	if rep.InputMappings != nil {
		task.inputMapper = data.NewMapper(rep.InputMappings)
	}

	if rep.OutputMappings != nil {
		task.outputMapper = data.NewMapper(rep.OutputMappings)
	}

	if len(rep.Attributes) > 0 {
		task.attrs = make(map[string]*data.Attribute, len(rep.Attributes))

		for _, value := range rep.Attributes {
			task.attrs[value.Name] = value
		}
	}

	def.tasks[task.id] = task
	numTasks := len(rep.Tasks)

	// flow child tasks
	if numTasks > 0 {

		for _, childTaskRep := range rep.Tasks {

			childTask := &Task{}
			childTask.parent = task
			task.tasks = append(task.tasks, childTask)
			addTask(def, childTask, childTaskRep)
		}
	}
}

func addLinks(def *Definition, task *Task, rep *TaskRep) {

	numLinks := len(rep.Links)

	if numLinks > 0 {

		task.links = make([]*Link, numLinks)

		for i, linkRep := range rep.Links {

			link := &Link{}
			link.id = linkRep.ID
			//link.Parent = task
			//link.Definition = pd
			link.linkType = LinkType(linkRep.Type)
			link.value = linkRep.Value
			link.fromTask = def.tasks[linkRep.FromID]
			link.toTask = def.tasks[linkRep.ToID]

			// add this link as predecessor "fromLink" to the "toTask"
			link.toTask.fromLinks = append(link.toTask.fromLinks, link)

			// add this link as successor "toLink" to the "fromTask"
			link.fromTask.toLinks = append(link.fromTask.toLinks, link)

			task.links[i] = link
			def.links[link.id] = link
		}
	}
}
