package filterprocessor

type Config struct {
	Metrics MetricFilters `mapstructure:"metrics"`
}

// MetricFilters filters by Metric properties.
type MetricFilters struct {
<<<<<<< HEAD
	MetricConditions    []string `mapstructure:"metric" yaml:"metric"`
	DataPointConditions []string `mapstructure:"datapoint" yaml:"datapoint"`
=======
	MetricConditions    []string `mapstructure:"metric" yaml:"metric,omitempty"`
	DataPointConditions []string `mapstructure:"datapoint" yaml:"datapoint,omitempty"`
>>>>>>> feat/agent-config-v1
}
