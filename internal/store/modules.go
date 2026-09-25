package store

// Features the admin can switch off to declutter the app (all on by
// default). Turning one off hides it everywhere and refuses its API calls;
// kids' content rules keep applying even when parent tools are hidden.
const (
	KeyModuleQueue    = "module_queue"          // Up Next reading queue
	KeyModuleKindle   = "module_send_to_kindle" // "Start Reading" by Send-to-Kindle email
	KeyModuleKOReader = "module_koreader"       // "Start Reading" flags for KOReader sync
	KeyModuleImport   = "module_import"         // Import books: Kindle/drive scanner and lists
	KeyModuleParents  = "module_parents"        // kids' accounts, age groups, parents' notes
	KeyModuleSuggest  = "module_suggestions"    // 💡 Suggested Reads under Up Next
	KeyNotifyRoutine  = "notify_routine"        // 🔔 also lists routine events, not just problems
)

// moduleNames are the short names the app uses for each switch.
var moduleNames = map[string]string{
	KeyModuleQueue:    "queue",
	KeyModuleKindle:   "send_to_kindle",
	KeyModuleKOReader: "koreader",
	KeyModuleImport:   "import",
	KeyModuleParents:  "parents",
	KeyModuleSuggest:  "suggestions",
}

func init() {
	for k := range moduleNames {
		Defaults[k] = "true"
	}
	Defaults[KeyNotifyRoutine] = "true"
}

// IsBoolSetting reports whether key holds true/false (module switches).
func IsBoolSetting(key string) bool {
	_, ok := moduleNames[key]
	return ok || key == KeyNotifyRoutine
}

// Modules reports which features are on, keyed by short name ("queue", …).
func (s *Store) Modules() map[string]bool {
	out := make(map[string]bool, len(moduleNames))
	for k, name := range moduleNames {
		out[name] = s.SettingBool(k)
	}
	return out
}
