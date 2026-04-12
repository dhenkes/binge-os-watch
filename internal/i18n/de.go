package i18n

func init() {
	Register(LangDE, map[string]string{
		// Nav
		"nav.library":     "Bibliothek",
		"nav.calendar":    "Kalender",
		"nav.discover":    "Entdecken",
		"nav.suggestions": "Vorschläge",
		"nav.search":      "Suche",
		"nav.settings":    "Einstellungen",
		"nav.stats":       "Statistiken",
		"nav.logout":      "Abmelden",

		// Actions
		"action.save":    "Speichern",
		"action.cancel":  "Abbrechen",
		"action.create":  "Erstellen",
		"action.change":  "Ändern",
		"action.edit":    "bearbeiten",
		"action.delete":  "löschen",
		"action.add":     "Hinzufügen",
		"action.remove":  "Entfernen",
		"action.dismiss": "Verwerfen",
		"action.search":  "Suchen",
		"action.confirm": "sicher?",

		// Media
		"media.movie":         "Film",
		"media.tv":            "Serie",
		"media.plan_to_watch": "Geplant",
		"media.watching":      "Schaue ich",
		"media.watched":       "Gesehen",
		"media.on_hold":       "Pausiert",
		"media.dropped":       "Abgebrochen",
		"media.add_to_lib":    "Zur Bibliothek hinzufügen",
		"media.in_library":    "Bereits in Bibliothek",
		"media.mark_watched":  "Als gesehen markieren",
		"media.mark_next":     "Nächste Folge markieren",
		"media.rate":          "Bewerten",
		"media.where_to_watch": "Wo schauen",

		// Library
		"library.title":        "Bibliothek",
		"library.empty":        "Deine Bibliothek ist leer. Suche etwas zum Hinzufügen.",
		"library.empty_filter": "Keine Einträge für diesen Filter.",
		"library.all":          "Alle",
		"library.sort":         "Sortieren",
		"library.sort_title":   "Titel",
		"library.sort_added":   "Hinzugefügt",
		"library.sort_release": "Erscheinungsdatum",
		"library.sort_rating":  "Bewertung",

		// Calendar
		"calendar.title":    "Kalender",
		"calendar.upcoming": "Demnächst",
		"calendar.recent":   "Kürzlich erschienen",
		"calendar.empty":    "Nichts geplant.",

		// Discover
		"discover.title":           "Entdecken",
		"discover.trending":        "Im Trend",
		"discover.popular":         "Beliebt",
		"discover.recommendations": "Empfehlungen",
		"discover.load_more":       "Mehr laden",

		// Search
		"search.title":   "Suche",
		"search.hint":    "Filme und Serien suchen...",
		"search.empty":   "Keine Ergebnisse gefunden.",

		// Tags
		"tags.title": "Tags",
		"tags.empty": "Noch keine Tags.",
		"tags.add":   "Tag hinzufügen",

		// Keyword Watches
		"keywords.title":       "Stichwort-Überwachung",
		"keywords.empty":       "Noch keine Stichwort-Überwachungen.",
		"keywords.suggestions": "Vorschläge",
		"keywords.dismiss_all": "Alle verwerfen",

		// Settings
		"settings.title":            "Einstellungen",
		"settings.appearance":       "Darstellung",
		"settings.theme":            "Design",
		"settings.language":         "Sprache",
		"settings.region":           "Region",
		"settings.region_hint":      "ISO 3166-1 Code für Streaming-Anbieter (z.B. DE, AT)",
		"settings.change_password":  "Passwort ändern",
		"settings.current_password": "Aktuelles Passwort",
		"settings.new_password":     "Neues Passwort",

		// Auth
		"auth.login":          "Anmelden",
		"auth.register":       "Registrieren",
		"auth.signin":         "In dein Konto einloggen",
		"auth.create_account": "Neues Konto erstellen",
		"auth.username":       "Benutzername",
		"auth.password":       "Passwort",
		"auth.first_time":     "Zum ersten Mal hier?",
		"auth.create_link":    "Konto erstellen",
		"auth.have_account":   "Bereits ein Konto?",
		"auth.login_link":     "Anmelden",

		// Logout
		"logout.title":   "Abmelden",
		"logout.confirm": "Bist du sicher, dass du dich abmelden möchtest?",
		"logout.yes":     "Ja, abmelden",

		// Admin
		"nav.admin":         "Admin",
		"admin.users_title": "Benutzer",
		"admin.stats_title": "Statistiken",

		// Stats
		"stats.title":          "Statistiken",
		"stats.total_movies":   "Filme gesehen",
		"stats.total_episodes": "Folgen gesehen",
		"stats.total_time":     "Gesamte Watchtime",
		"stats.avg_rating":     "Durchschnittsbewertung",
		"stats.genres":         "Genres",
		"stats.rating_dist":    "Bewertungsverteilung",
		"stats.monthly":        "Monatliche Aktivität",
		"stats.streak":         "Aktuelle Serie",
		"stats.longest_streak": "Längste Serie",
		"stats.days":           "Tage",

		// Settings (new sections)
		"settings.webhooks":       "Webhooks",
		"settings.webhooks_empty": "Keine Webhooks konfiguriert.",
		"settings.webhook_url":    "Webhook URL",
		"settings.ics":            "ICS Kalender",
		"settings.ics_hint":       "Abonniere deinen Kalender in jeder App, die iCalendar-Feeds unterstützt.",
		"settings.ics_regenerate": "Token neu generieren",
		"settings.ics_none":       "Noch kein ICS-Token generiert.",
		"settings.trakt":          "Trakt Import",
		"settings.trakt_hint":     "Lade einen Trakt JSON-Export hoch. Filme werden als gesehen markiert. Serien werden zur Bibliothek hinzugefügt (Folgenfortschritt muss manuell nachgetragen werden).",

		// Admin (extended)
		"admin.role":        "Rolle",
		"admin.delete_user": "Benutzer löschen",
	})
}
