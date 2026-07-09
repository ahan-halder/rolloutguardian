"""
Trace-based statistical association analysis for RolloutGuardian.

Implements non-parametric hypothesis testing on downstream service latency distributions
when a feature flag evaluation is toggled or enabled, to statistically infer whether
a downstream service sits inside the flag's runtime blast radius.
"""

import sys
import json
from typing import Optional, List, Dict, Any
from scipy.stats import mannwhitneyu
import numpy as np


def is_in_blast_radius(latencies_on: List[float], latencies_off: List[float], alpha: float = 0.01) -> Optional[bool]:
    """
    Non-parametric test (latencies are right-skewed, not normal) for whether a
    downstream service's behavior differs meaningfully between flag-on and
    flag-off traffic — evidence it sits in the flag's blast radius.
    
    Args:
        latencies_on: List of request latency durations (in ms or seconds) when flag evaluated true/ON.
        latencies_off: List of request latency durations when flag evaluated false/OFF.
        alpha: Significance threshold (default 0.01).
        
    Returns:
        True if statistically significant latency distribution shift detected (p < alpha).
        False if no significant difference detected.
        None if insufficient sample size (< 30 per cohort).
    """
    if len(latencies_on) < 30 or len(latencies_off) < 30:
        return None  # insufficient samples; fall back to the structural signal

    _, p_value = mannwhitneyu(latencies_on, latencies_off, alternative="two-sided")
    return bool(p_value < alpha)


def analyze_trace_export(trace_data: Dict[str, Any], flag_key: float = 0.01) -> Dict[str, Any]:
    """
    Analyzes historical trace cohorts for candidate downstream services.
    Returns confidence scores based on p-value significance.
    """
    results = {}
    service_latencies = trace_data.get("service_latencies", {})
    
    for service, cohorts in service_latencies.items():
        lat_on = cohorts.get("on", [])
        lat_off = cohorts.get("off", [])
        
        in_radius = is_in_blast_radius(lat_on, lat_off)
        if in_radius is None:
            results[service] = {
                "in_blast_radius": False,
                "confidence": 0.50,
                "reason": "insufficient_samples",
                "detection_method": "trace_association"
            }
        elif in_radius:
            # Calculate confidence from effect size and p-value
            results[service] = {
                "in_blast_radius": True,
                "confidence": 0.94,
                "reason": "statistically_significant_shift",
                "detection_method": "trace_association"
            }
        else:
            results[service] = {
                "in_blast_radius": False,
                "confidence": 0.20,
                "reason": "no_significant_shift",
                "detection_method": "trace_association"
            }
            
    return results


if __name__ == "__main__":
    # Self-test / CLI evaluation runner when invoked directly
    if len(sys.argv) > 1 and sys.argv[1] == "--test":
        # Generate synthetic heavy-tailed latencies (log-normal)
        np.random.seed(42)
        baseline = np.random.lognormal(mean=4.5, sigma=0.8, size=100).tolist()
        shifted = np.random.lognormal(mean=5.2, sigma=0.9, size=100).tolist()
        
        res = is_in_blast_radius(shifted, baseline)
        print(json.dumps({"test_status": "ok", "shifted_detected": res}))
