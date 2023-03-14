package opamp

import (
	"fmt"
	"sync"

	"go.uber.org/zap"
)

var lockTracesPipelineSpec sync.RWMutex
var lockMetricsPipelineSpec sync.RWMutex

type pipelineElement struct {
	Name string

	// disabled when set to true,  removes the processor the agent pipeline
	// not used when Anchor = true
	Disabled bool

	// indicates the element will not alter agent pipeline but
	// will be used just to position other processors
	// e.g. In ["batch", "signoz_tail_sampling"], we would want
	// to use batch as anchor such that even if batch does not exist in
	// agent pipeline, we dont make an effort to add it. neither
	// we remove batch from the agent pipeline if it is disabled here
	// we just use position of batch (0) to put signoz_tail_sampling
	// next to it
	Anchor bool
}

var tracesPipelineSpec = map[int]pipelineElement{
	0: {
		Name: "signoz_tail_sampling",
		// will be disabled, until first sampling rule is set
		Disabled: true,
	},
	1: {
		Name: "batch",
		// if batch processor doesnt exist in agent pipeline
		// we dont want to force add it.
		Anchor: true,
	},
}

var metricsPipelineSpec = map[int]pipelineElement{
	0: {
		Name: "filter",
		// will be disabled, until first drop rule is set
		Disabled: true,
	},
	1: {
		Name: "batch",
		// if batch processor doesnt exist in agent pipeline
		// we dont want to force add it
		Anchor: true,
	},
}

// updatePipelineSpec would be important mainly for scenarios
// where default condition of  processor is false (e.g. signoz_tail_sampling)
func updatePipelineSpec(signal string, name string, enabled bool) {
	switch signal {
	case "metrics":
		lockMetricsPipelineSpec.Lock()
		defer lockMetricsPipelineSpec.Unlock()

		for i := 0; i < len(metricsPipelineSpec); i++ {
			p := metricsPipelineSpec[i]
			if p.Name == name {
				p.Disabled = !enabled
				metricsPipelineSpec[i] = p
			}
		}
	case "traces":
		lockTracesPipelineSpec.Lock()
		defer lockTracesPipelineSpec.Unlock()

		for i := 0; i < len(tracesPipelineSpec); i++ {
			p := tracesPipelineSpec[i]
			if p.Name == name {
				p.Disabled = !enabled
				tracesPipelineSpec[i] = p
			}
		}
	default:
		return
	}

}

// AddToTracePipeline to enable processor in traces pipeline
func AddToTracePipelineSpec(processor string) {
	updatePipelineSpec("traces", processor, true)
}

// RemoveFromTracePipeline to remove processor from traces pipeline
func RemoveFromTracePipelineSpec(name string) {
	updatePipelineSpec("traces", name, false)
}

// AddToMetricsPipeline to enable processor in traces pipeline
func AddToMetricsPipelineSpec(processor string) {
	updatePipelineSpec("metrics", processor, true)
}

// RemoveFromMetricsPipeline to remove processor from traces pipeline
func RemoveFromMetricsPipelineSpec(name string) {
	updatePipelineSpec("metrics", name, false)
}

func checkDuplicates(pipeline []interface{}) bool {
	exists := make(map[string]bool, len(pipeline))
	zap.S().Debugf("checking duplicate processors in the pipeline:", pipeline)
	for _, processor := range pipeline {
		name := processor.(string)
		if _, ok := exists[name]; ok {
			return true
		}

		exists[name] = true
	}
	return false
}

func buildPipeline(signal Signal, current []interface{}) ([]interface{}, error) {
	var spec map[int]pipelineElement

	switch signal {
	case Metrics:
		spec = metricsPipelineSpec
		lockMetricsPipelineSpec.Lock()
		defer lockMetricsPipelineSpec.Unlock()
	case Traces:
		spec = tracesPipelineSpec
		lockTracesPipelineSpec.Lock()
		defer lockTracesPipelineSpec.Unlock()
	default:
		return nil, fmt.Errorf("invalid signal")
	}

	pipeline := current

	// create a reverse map of existing config processors and their position
	// e.g. [spanmetrics, batch] will become {batch: 1, spanmetrics: 0}
	existing := map[string]int{}
	for i, p := range current {
		name := p.(string)
		existing[name] = i
	}

	// create mapping from our tracesPipelinePlan (processors managed by us) to position in existing processors (from current config)
	// for example: when
	// 		colletor config: ["spanmetrics", "batch"]
	// 		tracesPipelineSpec: ["signoz_tail_sampling", "batch"]
	// the map result will be
	// 		{"batch": 1}
	// the processor from our spec signoz_tail_sampling is missing
	// in collector config, so it will not have an entry in this map
	specVsExistingMap := map[int]int{}

	// go through plan and map its elements to current positions in effective config
	for i, m := range spec {
		if loc, ok := existing[m.Name]; ok {
			specVsExistingMap[i] = loc
		}
	}

	// lastMatched is pointer to last element from spec that
	// matched to current config in collector.
	lastMatched := -1

	// inserts keeps track of new additions that happen in the
	// processor list. it is useful in situations like below:
	// we store map of our expected spec vs current collector pipeline
	// in specVsExistingMap.  but, during the below loop
	// we also add elements to collector pipeline, hence
	// we need the indexes from specVsExistingMap to include new inserts
	inserts := 0

	// go through processer from pipeline spec (our expected order)
	// in the increasing order
	for i := 0; i < len(spec); i++ {
		m := spec[i]

		if loc, ok := specVsExistingMap[i]; ok {
			// processor already exists in the config

			// inserts accomodate for already made changes in prior loop
			currentPos := loc + inserts

			// if disabled then remove from the pipeline
			if m.Disabled {
				zap.S().Debug("build_pipeline: found a disabled item, removing from pipeline at position", currentPos-1, " ", m.Name)
				if currentPos-1 <= 0 {
					pipeline = pipeline[currentPos+1:]
				} else {
					pipeline = append(pipeline[:currentPos-1], pipeline[currentPos+1:]...)
				}
			}

			// capture last position where match was found,  this will be used
			// to insert missing elements next to it
			lastMatched = currentPos

		} else {
			if !m.Anchor {
				// track inserts as they shift the elements in pipeline
				inserts++

				// we use last matched to insert new item.  This means, we keep inserting missing processors
				// right after last matched processsor (e.g. insert filters after tail_sampling for existing list of [batch, tail_sampling])

				if lastMatched <= 0 {
					zap.S().Debug("build_pipeline: found a new item to be inserted, inserting at position 0:", m.Name)
					pipeline = append([]interface{}{m.Name}, pipeline[lastMatched+1:]...)
				} else {
					zap.S().Debug("build_pipeline: found a new item to be inserted, inserting at position :", lastMatched, " ", m.Name)
					prior := make([]interface{}, len(pipeline[:lastMatched]))
					next := make([]interface{}, len(pipeline[lastMatched:]))
					copy(prior, pipeline[:lastMatched])
					copy(next, pipeline[lastMatched:])

					pipeline = append(prior, m.Name)
					pipeline = append(pipeline, next...)
				}
			}
		}
	}

	if checkDuplicates(pipeline) {
		// duplicates are most likely because the processor sequence in effective config conflicts
		// with the planned sequence as per planned pipeline
		return pipeline, fmt.Errorf("the effective config has an unexpected processor sequence: %v", pipeline)
	}

	return pipeline, nil
}
