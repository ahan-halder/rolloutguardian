package harnessclient

// FME models
type Flag struct {
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	OwnerService string          `json:"owner_service"`
	Allocations  []Allocation    `json:"allocations"`
	Targeting    []TargetingRule `json:"targeting"`
}

type Allocation struct {
	Treatment  string  `json:"treatment"`
	Percentage float64 `json:"percentage"`
}

type TargetingRule struct {
	Name      string `json:"name"`
	Attribute string `json:"attribute"`
	Operator  string `json:"operator"`
	Values    []string `json:"values"`
}

type RolloutChangeEvent struct {
	FlagKey          string  `json:"flag_key"`
	RequestedChange  string  `json:"requested_change"` // e.g. "increase_rollout"
	FromPercentage   float64 `json:"from_pct"`
	ToPercentage     float64 `json:"to_pct"`
	RequesterEmail   string  `json:"requester_email"`
}

// Chaos Engineering models
type ChaosCoverageSummary struct {
	ServiceName          string   `json:"service_name"`
	DaysSinceLastResult  int      `json:"days_since_last_result"`
	ResilienceScore      *float64 `json:"resilience_score"`
	LastExperimentID     string   `json:"last_experiment_id"`
}

type K8sTargetInfo struct {
	Namespace     string `json:"namespace"`
	LabelSelector string `json:"label_selector"`
	ClusterID     string `json:"cluster_id"`
}

// SRM models
type SRMSummary struct {
	ServiceName             string  `json:"service_name"`
	SLO                     string  `json:"slo"`
	ErrorBudgetRemainingPct float64 `json:"error_budget_remaining_pct"`
	BurnRate24H             float64 `json:"burn_rate_24h"`
}

// STO models
type STOSummary struct {
	ServiceName  string `json:"service_name"`
	OpenCritical int    `json:"open_critical"`
	OpenHigh     int    `json:"open_high"`
}

// IDP Catalog models
type CatalogService struct {
	Name         string         `json:"name"`
	FlagKeys     []string       `json:"flag_keys"`
	Owner        string         `json:"owner"`
	Dependencies []string       `json:"dependencies"`
	K8sTarget    *K8sTargetInfo `json:"k8s_target,omitempty"`
}

type CatalogFixture struct {
	Services []CatalogService `json:"services"`
}

type ChaosMapFixture struct {
	ResilienceData map[string]ServiceResilienceRecord `json:"resilience_data"`
}

type ServiceResilienceRecord struct {
	Chaos ChaosRecord `json:"chaos"`
	SRM   SRMRecord   `json:"srm"`
	STO   STORecord   `json:"sto"`
}

type ChaosRecord struct {
	DaysSinceLastResult int      `json:"days_since_last_result"`
	ResilienceScore     *float64 `json:"resilience_score"`
	LastExperimentID    string   `json:"last_experiment_id"`
}

type SRMRecord struct {
	SLO                     string  `json:"slo"`
	ErrorBudgetRemainingPct float64 `json:"error_budget_remaining_pct"`
	BurnRate24H             float64 `json:"burn_rate_24h"`
}

type STORecord struct {
	OpenCritical int `json:"open_critical"`
	OpenHigh     int `json:"open_high"`
}
