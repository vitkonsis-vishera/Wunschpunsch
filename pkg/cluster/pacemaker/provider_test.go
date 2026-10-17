package pacemaker

import (
	"testing"
)

func TestParseCrmMonXML(t *testing.T) {
	xmlSample := []byte(`
<crm_mon>
  <summary>
    <nodes_configured number="2"/>
    <resources_configured number="2"/>
  </summary>
  <nodes>
    <node name="node1" id="1" online="true" standby="false"/>
    <node name="node2" id="2" online="true" standby="false"/>
  </nodes>
  <resources>
    <master id="pgsql-master">
      <resource id="pgsql" resource_id="pgsql" role="Master" active="true" node="node1"/>
    </master>
  </resources>
</crm_mon>
`)

	p := NewPacemakerProvider("localhost", "root")
	status, err := p.parseCrmMonXML(xmlSample)
	if err != nil {
		t.Fatalf("unexpected error parsing crm_mon xml: %v", err)
	}

	if status.EngineName != "Corosync + Pacemaker" {
		t.Errorf("expected EngineName 'Corosync + Pacemaker', got '%s'", status.EngineName)
	}

	if len(status.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(status.Nodes))
	}

	if status.Nodes[0].Name != "node1" || status.Nodes[0].Role != "primary" {
		t.Errorf("expected node1 to be primary, got %s (%s)", status.Nodes[0].Name, status.Nodes[0].Role)
	}

	if status.Nodes[1].Name != "node2" || status.Nodes[1].Role != "replica" {
		t.Errorf("expected node2 to be replica, got %s (%s)", status.Nodes[1].Name, status.Nodes[1].Role)
	}
}
