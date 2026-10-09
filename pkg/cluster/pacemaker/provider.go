package pacemaker

import (
	"context"
	"fmt"
	"os/exec"
	"wunschpunsch/pkg/cluster"
)

type PacemakerProvider struct {
	host     string
	sshUser  string
	pcsdPort int
}

func NewPacemakerProvider(host string, sshUser string) *PacemakerProvider {
	return &PacemakerProvider{
		host:     host,
		sshUser:  sshUser,
		pcsdPort: 2224,
	}
}

func (p *PacemakerProvider) Engine() string {
	return "Corosync + Pacemaker"
}

func (p *PacemakerProvider) GetStatus(ctx context.Context) (*cluster.ClusterStatus, error) {
	// Вызов crm_mon -X для получения XML структуры состояния кластера Corosync/Pacemaker
	cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", p.sshUser, p.host), "crm_mon -X")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to query pacemaker status: %w", err)
	}

	return p.parseCrmMonXML(output)
}

func (p *PacemakerProvider) parseCrmMonXML(xmlData []byte) (*cluster.ClusterStatus, error) {
	// Парсинг ресурса PostgreSQL (pgsql / pgsql-ms / PAF - PostgreSQL Automatic Failover)
	// Преобразование XML-нод Pacemaker в унифицированную структуру cluster.ClusterStatus
	status := &cluster.ClusterStatus{
		EngineName: p.Engine(),
		Scope:      "pacemaker-cluster",
		Nodes:      []cluster.NodeStatus{},
	}

	// Логика разбора ресурсов Master/Slave в Pacemaker...
	return status, nil
}

func (p *PacemakerProvider) Switchover(ctx context.Context, currentPrimary, candidate string) error {
	// Выполнение crm resource move / pcs resource move
	cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", p.sshUser, p.host),
		fmt.Sprintf("pcs resource move pgsql-master %s", candidate))
	return cmd.Run()
}

func (p *PacemakerProvider) Reinitialize(ctx context.Context, targetNode string) error {
	// Очистка состояния отказа для ноды: pcs resource cleanup
	cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", p.sshUser, p.host),
		fmt.Sprintf("pcs resource cleanup pgsql-master %s", targetNode))
	return cmd.Run()
}

func (p *PacemakerProvider) SetMaintenance(ctx context.Context, enabled bool) error {
	mode := "off"
	if enabled {
		mode = "on"
	}
	cmd := exec.CommandContext(ctx, "ssh", fmt.Sprintf("%s@%s", p.sshUser, p.host),
		fmt.Sprintf("pcs property set maintenance-mode=%s", mode))
	return cmd.Run()
}
