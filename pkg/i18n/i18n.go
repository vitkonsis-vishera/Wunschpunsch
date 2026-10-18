package i18n

type Language string

const (
	EN Language = "en"
	RU Language = "ru"
)

type Translation struct {
	ConnectTab    string
	TopologyTab   string
	MetricsTab    string
	AnalyticsTab  string
	ActionsTab    string
	LogsTab       string
	SettingsTab   string
	ExportSuccess string
	ExportError   string
	ConnStatus    string
	SearchPrompt  string
	SettingsTitle string
	SettingsHint  string
	MetricsConfig string
	FileSaved     string
}

var translations = map[Language]Translation{
	EN: {
		ConnectTab:    "0: Connect",
		TopologyTab:   "1: Topology",
		MetricsTab:    "2: Metrics",
		AnalyticsTab:  "3: Analytics",
		ActionsTab:    "4: Actions",
		LogsTab:       "5: Event Logs",
		SettingsTab:   "6: Settings",
		ExportSuccess: "Data exported successfully!",
		ExportError:   "Failed to export data",
		ConnStatus:    "Connected to cluster",
		SearchPrompt:  "🔍 Search: ",
		SettingsTitle: "Global App Settings",
		SettingsHint:  "Up/Down: Select Field  •  Type to edit  •  Enter: Save",
		MetricsConfig: "Configure displayed metrics (Space — toggle, Esc — exit)",
		FileSaved:     "File saved to path:",
	},
	RU: {
		ConnectTab:    "0: Подключение",
		TopologyTab:   "1: Топология",
		MetricsTab:    "2: Метрики",
		AnalyticsTab:  "3: Аналитика",
		ActionsTab:    "4: Действия",
		LogsTab:       "5: Логи событий",
		SettingsTab:   "6: Настройки",
		ExportSuccess: "Данные успешно экспортированы!",
		ExportError:   "Ошибка при экспорте данных",
		ConnStatus:    "Подключено к кластеру",
		SearchPrompt:  "🔍 Поиск: ",
		SettingsTitle: "Глобальные настройки (Global Settings)",
		SettingsHint:  "Up/Down: Выбор поля  •  Буквы/Цифры/Стрелки: Редактирование текста  •  Enter: Сохранить",
		MetricsConfig: "Настройка отображаемых метрик (Space — вкл/выкл, Esc — выход)",
		FileSaved:     "Файл сохранен по пути:",
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
