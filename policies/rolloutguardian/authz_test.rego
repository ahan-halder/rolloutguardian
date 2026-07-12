package rolloutguardian

test_allow_healthy_service {
	input_payload := {
		"blast_radius": [
			{
				"name": "fraud-check-service",
				"chaos": {"days_since_last_result": 14, "resilience_score": 0.92},
				"srm": {"error_budget_remaining_pct": 61.2},
				"sto": {"open_critical": 0},
			}
		],
		"signals_config": {"sto": {"block_on_open_critical": true}}
	}

	decision == "allow" with input as input_payload
	count(reasons) == 0 with input as input_payload
}

test_block_stale_chaos_and_low_budget {
	input_payload := {
		"blast_radius": [
			{
				"name": "payment-service",
				"chaos": {"days_since_last_result": 118, "resilience_score": 0.50},
				"srm": {"error_budget_remaining_pct": 8.5},
				"sto": {"open_critical": 0},
			}
		],
		"signals_config": {"sto": {"block_on_open_critical": true}}
	}

	decision == "block" with input as input_payload
	count(block) == 1 with input as input_payload
}

test_warn_low_budget_only {
	input_payload := {
		"blast_radius": [
			{
				"name": "checkout-service",
				"chaos": {"days_since_last_result": 10, "resilience_score": 0.88},
				"srm": {"error_budget_remaining_pct": 18.5},
				"sto": {"open_critical": 0},
			}
		],
		"signals_config": {"sto": {"block_on_open_critical": true}}
	}

	decision == "warn" with input as input_payload
	count(warn) == 1 with input as input_payload
}

test_block_on_open_critical_sto {
	input_payload := {
		"blast_radius": [
			{
				"name": "auth-service",
				"chaos": {"days_since_last_result": 10, "resilience_score": 0.95},
				"srm": {"error_budget_remaining_pct": 80.0},
				"sto": {"open_critical": 2},
			}
		],
		"signals_config": {"sto": {"block_on_open_critical": true}}
	}

	decision == "block" with input as input_payload
	count(block) == 1 with input as input_payload
}
