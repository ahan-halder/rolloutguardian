package harnessclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type MockClient struct {
	CatalogServices []CatalogService
	ResilienceMap   map[string]ServiceResilienceRecord
}

func NewMockClient(catalogPath, chaosMapPath string) (*MockClient, error) {
	m := &MockClient{
		ResilienceMap: make(map[string]ServiceResilienceRecord),
	}

	// Try reading catalog
	catalogData, err := loadFixtureFile(catalogPath, "catalog.json")
	if err == nil {
		var fixture CatalogFixture
		if err := json.Unmarshal(catalogData, &fixture); err == nil {
			m.CatalogServices = fixture.Services
		}
	}
	if len(m.CatalogServices) == 0 {
		m.CatalogServices = defaultCatalogServices()
	}

	// Try reading chaos map
	chaosData, err := loadFixtureFile(chaosMapPath, "chaos-map.json")
	if err == nil {
		var fixture ChaosMapFixture
		if err := json.Unmarshal(chaosData, &fixture); err == nil {
			m.ResilienceMap = fixture.ResilienceData
		}
	}
	if len(m.ResilienceMap) == 0 {
		m.ResilienceMap = defaultResilienceMap()
	}

	return m, nil
}

func loadFixtureFile(providedPath, filename string) ([]byte, error) {
	candidates := []string{
		providedPath,
		"examples/catalog-fixtures/" + filename,
		"../../examples/catalog-fixtures/" + filename,
		"../../../examples/catalog-fixtures/" + filename,
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("fixture file %s not found", filename)
}

func defaultCatalogServices() []CatalogService {
	return []CatalogService{
		{
			Name:         "checkout-service",
			FlagKeys:     []string{"checkout-v2-express-pay"},
			Owner:        "checkout-team",
			Dependencies: []string{"payment-service", "fraud-check-service"},
		},
		{
			Name:         "payment-service",
			FlagKeys:     []string{},
			Owner:        "payments-platform",
			Dependencies: []string{"ledger-service"},
			K8sTarget: &K8sTargetInfo{
				Namespace:     "payments",
				LabelSelector: "app=payment-service",
				ClusterID:     "prod-payments-cluster",
			},
		},
		{
			Name:         "fraud-check-service",
			FlagKeys:     []string{},
			Owner:        "risk-team",
			Dependencies: []string{},
			K8sTarget: &K8sTargetInfo{
				Namespace:     "risk",
				LabelSelector: "app=fraud-check",
				ClusterID:     "prod-risk-cluster",
			},
		},
	}
}

func defaultResilienceMap() map[string]ServiceResilienceRecord {
	score92 := 0.92
	return map[string]ServiceResilienceRecord{
		"payment-service": {
			Chaos: ChaosRecord{DaysSinceLastResult: 118, ResilienceScore: nil, LastExperimentID: "exp-2026-03-12"},
			SRM:   SRMRecord{SLO: "payment-service-availability", ErrorBudgetRemainingPct: 12.4, BurnRate24H: 2.8},
			STO:   STORecord{OpenCritical: 0, OpenHigh: 1},
		},
		"fraud-check-service": {
			Chaos: ChaosRecord{DaysSinceLastResult: 14, ResilienceScore: &score92, LastExperimentID: "exp-2026-06-25"},
			SRM:   SRMRecord{SLO: "fraud-check-availability", ErrorBudgetRemainingPct: 61.2, BurnRate24H: 0.9},
			STO:   STORecord{OpenCritical: 0, OpenHigh: 0},
		},
	}
}

func (m *MockClient) GetCatalogServices(ctx context.Context) ([]CatalogService, error) {
	return m.CatalogServices, nil
}

func (m *MockClient) GetChaosCoverage(ctx context.Context, serviceName string) (*ChaosCoverageSummary, error) {
	record, ok := m.ResilienceMap[serviceName]
	if !ok {
		// Return default fresh coverage if not found in map
		score := 0.85
		return &ChaosCoverageSummary{
			ServiceName:         serviceName,
			DaysSinceLastResult: 30,
			ResilienceScore:     &score,
			LastExperimentID:    "exp-default",
		}, nil
	}
	return &ChaosCoverageSummary{
		ServiceName:         serviceName,
		DaysSinceLastResult: record.Chaos.DaysSinceLastResult,
		ResilienceScore:     record.Chaos.ResilienceScore,
		LastExperimentID:    record.Chaos.LastExperimentID,
	}, nil
}

func (m *MockClient) GetSRMSummary(ctx context.Context, serviceName string) (*SRMSummary, error) {
	record, ok := m.ResilienceMap[serviceName]
	if !ok {
		return &SRMSummary{
			ServiceName:             serviceName,
			SLO:                     serviceName + "-availability",
			ErrorBudgetRemainingPct: 80.0,
			BurnRate24H:             0.5,
		}, nil
	}
	return &SRMSummary{
		ServiceName:             serviceName,
		SLO:                     record.SRM.SLO,
		ErrorBudgetRemainingPct: record.SRM.ErrorBudgetRemainingPct,
		BurnRate24H:             record.SRM.BurnRate24H,
	}, nil
}

func (m *MockClient) GetSTOSummary(ctx context.Context, serviceName string) (*STOSummary, error) {
	record, ok := m.ResilienceMap[serviceName]
	if !ok {
		return &STOSummary{
			ServiceName:  serviceName,
			OpenCritical: 0,
			OpenHigh:     0,
		}, nil
	}
	return &STOSummary{
		ServiceName:  serviceName,
		OpenCritical: record.STO.OpenCritical,
		OpenHigh:     record.STO.OpenHigh,
	}, nil
}

func (m *MockClient) GetFlagDetails(ctx context.Context, flagKey string) (*Flag, error) {
	for _, svc := range m.CatalogServices {
		for _, fk := range svc.FlagKeys {
			if fk == flagKey {
				return &Flag{
					Key:          flagKey,
					Name:         flagKey,
					OwnerService: svc.Name,
				}, nil
			}
		}
	}
	return &Flag{
		Key:          flagKey,
		Name:         flagKey,
		OwnerService: "checkout-service",
	}, nil
}
