package i18n

import (
	"testing"
)

func TestNewLocalizerAndToggle(t *testing.T) {
	l := NewLocalizer(EN)
	if l.CurrentLang != EN {
		t.Errorf("expected EN, got %s", l.CurrentLang)
	}

	tr := l.T()
	if tr.TopologyTab != "1: Topology" {
		t.Errorf("expected '1: Topology', got %s", tr.TopologyTab)
	}

	l.Toggle()
	if l.CurrentLang != RU {
		t.Errorf("expected RU after toggle, got %s", l.CurrentLang)
	}

	trRU := l.T()
	if trRU.TopologyTab != "1: Топология" {
		t.Errorf("expected '1: Топология', got %s", trRU.TopologyTab)
	}

	l.Toggle()
	if l.CurrentLang != EN {
		t.Errorf("expected EN after second toggle, got %s", l.CurrentLang)
	}
}
