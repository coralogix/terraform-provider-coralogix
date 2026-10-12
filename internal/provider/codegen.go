package provider

// The resources that tools/iac-codegen generates. Each line states the resource and its
// directory in internal/provider/generated. `go generate ./internal/provider/` regenerates
// them all. The CI version check of the generator reads these lines to know what to compare.

//go:generate make -C ../.. codegen RESOURCE=AiEvaluation OUT=aievaluation
//go:generate make -C ../.. codegen RESOURCE=AlertSchedulerRule OUT=alertsscheduler
//go:generate make -C ../.. codegen RESOURCE=ConfigurationGroup OUT=configurationgroup
//go:generate make -C ../.. codegen RESOURCE=Connector OUT=connector
//go:generate make -C ../.. codegen RESOURCE=E2M OUT=events2metric
//go:generate make -C ../.. codegen RESOURCE=GlobalRouter OUT=globalrouter
//go:generate make -C ../.. codegen RESOURCE=Preset OUT=preset
//go:generate make -C ../.. codegen RESOURCE=Slo OUT=slov2
//go:generate make -C ../.. codegen RESOURCE=View OUT=view
//go:generate make -C ../.. codegen RESOURCE=ViewFolder OUT=viewfolder
//go:generate make -C ../.. codegen RESOURCE=v2.Target OUT=archivelogs
