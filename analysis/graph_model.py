"""
Structural Dependency Graph Model for RolloutGuardian.

Uses NetworkX to build a unified graph merging static call-site attribution with
1-to-2 hop dependency neighborhoods from IDP Backstage software catalog and
Harness Chaos application topology maps.
"""

import json
import networkx as nx
from typing import List, Dict, Any


class BlastRadiusGraphModel:
    def __init__(self):
        self.graph = nx.DiGraph()

    def load_from_catalog(self, catalog_json: Dict[str, Any]) -> None:
        """Populates the directed dependency graph from catalog entities."""
        services = catalog_json.get("services", [])
        for svc in services:
            name = svc.get("name")
            if not name:
                continue
            self.graph.add_node(name, owner=svc.get("owner"), flag_keys=svc.get("flag_keys", []))
            for dep in svc.get("dependencies", []):
                self.graph.add_edge(name, dep, edge_type="structural_catalog", base_weight=0.65)

    def add_static_callsites(self, service_callsites: Dict[str, List[str]]) -> None:
        """Incorporates tree-sitter or AST-derived static call-site ownership edges."""
        for flag_key, caller_services in service_callsites.items():
            for svc in caller_services:
                if not self.graph.has_node(svc):
                    self.graph.add_node(svc, flag_keys=[flag_key])
                else:
                    flags = self.graph.nodes[svc].get("flag_keys", [])
                    if flag_key not in flags:
                        flags.append(flag_key)
                        self.graph.nodes[svc]["flag_keys"] = flags

    def resolve_neighborhood(self, owner_service: str, max_hops: int = 2) -> List[Dict[str, Any]]:
        """Walks the graph outward from owner_service to compute confidence scores."""
        if not self.graph.has_node(owner_service):
            return []

        resolved = []
        lengths = nx.single_source_shortest_path_length(self.graph, owner_service, cutoff=max_hops)
        
        for node, distance in lengths.items():
            if distance == 0:
                continue  # Skip the owning service itself
            
            confidence = 0.81 if distance == 1 else 0.65
            detection_method = "static_callsite" if distance == 1 else "structural_neighborhood"
            
            resolved.append({
                "service": node,
                "confidence": confidence,
                "detection_method": detection_method,
                "hop_distance": distance
            })
            
        return resolved


if __name__ == "__main__":
    # Quick demo verification when run directly
    model = BlastRadiusGraphModel()
    mock_catalog = {
        "services": [
            {"name": "checkout-service", "dependencies": ["payment-service", "fraud-check-service"]},
            {"name": "payment-service", "dependencies": ["ledger-service"]},
            {"name": "fraud-check-service", "dependencies": []}
        ]
    }
    model.load_from_catalog(mock_catalog)
    nodes = model.resolve_neighborhood("checkout-service", max_hops=2)
    print(json.dumps(nodes, indent=2))
