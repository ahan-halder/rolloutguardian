package rolloutguardian

default_coverage_window_days := 90
min_healthy_budget_pct := 25.0
min_marginal_budget_pct := 15.0

# Block conditions: Stale Chaos coverage + Critical/Marginal Error Budget
block[msg] {
	svc := input.blast_radius[_]
	svc.chaos.days_since_last_result > default_coverage_window_days
	svc.srm.error_budget_remaining_pct < min_marginal_budget_pct
	msg := sprintf(
		"%s has no chaos coverage within %d days AND error budget remaining (%.1f%%) is below the marginal threshold (%.1f%%)",
		[svc.name, default_coverage_window_days, svc.srm.error_budget_remaining_pct, min_marginal_budget_pct],
	)
}

# Block conditions: Open Critical STO Findings (if enabled)
block[msg] {
	svc := input.blast_radius[_]
	input.signals_config.sto.block_on_open_critical == true
	svc.sto.open_critical > 0
	msg := sprintf(
		"%s has %d open critical security finding(s)",
		[svc.name, svc.sto.open_critical],
	)
}

# Warn conditions: Depleted error budget above marginal threshold
warn[msg] {
	count(block) == 0
	svc := input.blast_radius[_]
	svc.srm.error_budget_remaining_pct < min_healthy_budget_pct
	svc.srm.error_budget_remaining_pct >= min_marginal_budget_pct
	msg := sprintf(
		"%s error budget remaining (%.1f%%) is below the healthy threshold (%.1f%%) — proceeding requires SRE awareness",
		[svc.name, svc.srm.error_budget_remaining_pct, min_healthy_budget_pct],
	)
}

# Warn conditions: Stale Chaos coverage with healthy error budget
warn[msg] {
	count(block) == 0
	svc := input.blast_radius[_]
	svc.chaos.days_since_last_result > default_coverage_window_days
	svc.srm.error_budget_remaining_pct >= min_healthy_budget_pct
	msg := sprintf(
		"%s has no chaos coverage within %d days — schedule a resilience test soon",
		[svc.name, default_coverage_window_days],
	)
}

default decision := "allow"
default reasons := []

decision := "block" {
	count(block) > 0
}

decision := "warn" {
	count(block) == 0
	count(warn) > 0
}

reasons := [m | m := block[_]] {
	count(block) > 0
}

reasons := [m | m := warn[_]] {
	count(block) == 0
	count(warn) > 0
}
