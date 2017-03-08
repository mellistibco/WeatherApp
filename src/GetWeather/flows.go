package main

import (
	"github.com/TIBCOSoftware/flogo-lib/core/action"
	"github.com/TIBCOSoftware/flogo-lib/engine"
	"github.com/TIBCOSoftware/flogo-lib/flow/flowinst"
	"github.com/TIBCOSoftware/flogo-lib/flow/service"
	"github.com/TIBCOSoftware/flogo-lib/flow/service/flowprovider"
	"github.com/TIBCOSoftware/flogo-lib/flow/service/staterecorder"
	"github.com/TIBCOSoftware/flogo-lib/flow/service/tester"
	"github.com/TIBCOSoftware/flogo-lib/flow/support"
)

var embeddedJSONFlows map[string]string

func init() {

	embeddedJSONFlows = make(map[string]string)

	embeddedJSONFlows["embedded://getWeather"] = "H4sIAAAJbogC/4ySQY/TMBCF7/yKas4mSbvSavENoRXigIR2I3FAPbj2kA51PMYeF0rV/46catNWCMF1/Pzmfc8+QjAjgoYB5TMa2WICBSM79KBBaGP5daYxegQFcogIeqnAiCTaFMEM+staQWKW3uQd6COQmyQXrRXakxz6aQCgXjbWS9XU5F21Od9czccfwp53uHh6fO4Xz5j2ZG8j3NqekybM1fIm3kw4omzZgYK98aUO3j/2syNkSRQGOKlZXxJdibciUbetidT8KMFhGhKX4BrLYx22aJdv0HVo7jYP9/cPXTsge+Zdia3l4EiIQ26/t/oXxXfssG2+ZQ5/7l8roBCLfDQxUhjOBDP1S5pj30STzJhP9bVM7Bk0RCPbT9MUTusKUgu9mwt9wugPi54XfaJhmN75X21Gf/hrnZYdXvWz6roLDAXBuuF/ad6umoS5eLnGcUZMBVkr8BSu/shSwdfE4/RZhCfEs2VXsQF/Rk+WZOIFLang6dVvAAAA//8BAAD//+JKRTvoAgAA"

}

// EnableFlowServices enables flow services and action for engine
func EnableFlowServices(engine *engine.Engine, engineConfig *engine.Config) {

	log.Debug("Flow Services and Actions enabled")

	embeddedFlowMgr := support.NewEmbeddedFlowManager(true, embeddedJSONFlows)

	fpConfig := engineConfig.Services[service.ServiceFlowProvider]
	flowProvider := flowprovider.NewRemoteFlowProvider(fpConfig, embeddedFlowMgr)
	engine.RegisterService(flowProvider)

	srConfig := engineConfig.Services[service.ServiceStateRecorder]
	stateRecorder := staterecorder.NewRemoteStateRecorder(srConfig)
	engine.RegisterService(stateRecorder)

	etConfig := engineConfig.Services[service.ServiceEngineTester]
	engineTester := tester.NewRestEngineTester(etConfig)
	engine.RegisterService(engineTester)

	options := &flowinst.ActionOptions{Record: stateRecorder.Enabled()}

	flowAction := flowinst.NewFlowAction(flowProvider, stateRecorder, options)
	action.Register(flowinst.ActionType, flowAction)
}
