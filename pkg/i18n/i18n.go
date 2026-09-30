package i18n

type Language string

const (
	EN Language = "en"
	RU Language = "ru"
)

type Translation struct {
	TopologyTab   string
	MetricsTab    string
	ActionsTab    string
	LogsTab       string
	SettingsTab   string
	ExportSuccess string
	ExportError   string
	ConnStatus    string
}

var translations = map[Language]Translation{
	EN: {
		TopologyTab:   "1: Topology",
		MetricsTab:    "2: Metrics",
		ActionsTab:    "3: Actions",
		LogsTab:       "4: Event Logs",
		SettingsTab:   "5: Settings",
		ExportSuccess: "Data exported successfully!",
		ExportError:   "Failed to export data",
		ConnStatus:    "Connected to cluster",
	},
	RU: {
		TopologyTab:   "1: Топология",
		MetricsTab:    "2: Метрики",
		ActionsTab:    "3: Действия",
		LogsTab:       "4: Логи событий",
		SettingsTab:   "5: Настройки",
		ExportSuccess: "Данные успешно экспортированы!",
		ExportError:   "Ошибка при экспорте данных",
		ConnStatus:    "Подключено к кластеру",
	},
}

type Localizer struct {
	CurrentLang Language
}

func NewLocalizer(lang Language) *Localizer {
	return &Localizer{CurrentLang: lang}
}

func (l *Localizer) Toggle() {
	if l.CurrentLang == EN {
		l.CurrentLang = RU
	} else {
		l.CurrentLang = EN
	}
}

func (l *Localizer) T() Translation {
	return translations[l.CurrentLang]
}
