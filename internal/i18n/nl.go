package i18n

func init() {
	Register(LangNL, map[string]string{
		// Nav
		"nav.library":     "Bibliotheek",
		"nav.calendar":    "Kalender",
		"nav.discover":    "Ontdekken",
		"nav.suggestions": "Suggesties",
		"nav.search":      "Zoeken",
		"nav.settings":    "Instellingen",
		"nav.stats":       "Statistieken",
		"nav.logout":      "Uitloggen",

		// Actions
		"action.save":    "Opslaan",
		"action.cancel":  "Annuleren",
		"action.create":  "Aanmaken",
		"action.change":  "Wijzigen",
		"action.edit":    "bewerken",
		"action.delete":  "verwijderen",
		"action.add":     "Toevoegen",
		"action.remove":  "Verwijderen",
		"action.dismiss": "Negeren",
		"action.search":  "Zoeken",
		"action.confirm": "zeker?",

		// Media
		"media.movie":         "Film",
		"media.tv":            "Serie",
		"media.plan_to_watch": "Nog te kijken",
		"media.watching":      "Aan het kijken",
		"media.watched":       "Gezien",
		"media.on_hold":       "Gepauzeerd",
		"media.dropped":       "Gestopt",
		"media.add_to_lib":    "Toevoegen aan bibliotheek",
		"media.in_library":    "Al in bibliotheek",
		"media.mark_watched":  "Markeer als gezien",
		"media.mark_next":     "Markeer volgende aflevering",
		"media.rate":          "Beoordelen",
		"media.where_to_watch": "Waar te kijken",

		// Library
		"library.title":        "Bibliotheek",
		"library.empty":        "Je bibliotheek is leeg. Zoek iets om toe te voegen.",
		"library.empty_filter": "Geen items voor dit filter.",
		"library.all":          "Alle",
		"library.sort":         "Sorteren",
		"library.sort_title":   "Titel",
		"library.sort_added":   "Datum toegevoegd",
		"library.sort_release": "Verschijningsdatum",
		"library.sort_rating":  "Beoordeling",

		// Calendar
		"calendar.title":    "Kalender",
		"calendar.upcoming": "Binnenkort",
		"calendar.recent":   "Recent verschenen",
		"calendar.empty":    "Niets gepland.",

		// Discover
		"discover.title":           "Ontdekken",
		"discover.trending":        "Trending",
		"discover.popular":         "Populair",
		"discover.recommendations": "Aanbevelingen",
		"discover.load_more":       "Meer laden",

		// Search
		"search.title":   "Zoeken",
		"search.hint":    "Films en series zoeken...",
		"search.empty":   "Geen resultaten gevonden.",

		// Tags
		"tags.title": "Tags",
		"tags.empty": "Nog geen tags.",
		"tags.add":   "Tag toevoegen",

		// Keyword Watches
		"keywords.title":       "Trefwoord-bewaking",
		"keywords.empty":       "Nog geen trefwoord-bewakingen.",
		"keywords.suggestions": "Suggesties",
		"keywords.dismiss_all": "Alles negeren",

		// Settings
		"settings.title":            "Instellingen",
		"settings.appearance":       "Weergave",
		"settings.theme":            "Thema",
		"settings.language":         "Taal",
		"settings.region":           "Regio",
		"settings.region_hint":      "ISO 3166-1 code voor streamingdiensten (bijv. NL, BE)",
		"settings.change_password":  "Wachtwoord wijzigen",
		"settings.current_password": "Huidig wachtwoord",
		"settings.new_password":     "Nieuw wachtwoord",

		// Auth
		"auth.login":          "Inloggen",
		"auth.register":       "Registreren",
		"auth.signin":         "Log in op je account",
		"auth.create_account": "Nieuw account aanmaken",
		"auth.username":       "Gebruikersnaam",
		"auth.password":       "Wachtwoord",
		"auth.first_time":     "Eerste keer?",
		"auth.create_link":    "Account aanmaken",
		"auth.have_account":   "Al een account?",
		"auth.login_link":     "Inloggen",

		// Logout
		"logout.title":   "Uitloggen",
		"logout.confirm": "Weet je zeker dat je wilt uitloggen?",
		"logout.yes":     "Ja, uitloggen",

		// Admin
		"nav.admin":         "Admin",
		"admin.users_title": "Gebruikers",
		"admin.stats_title": "Statistieken",

		// Stats
		"stats.title":          "Statistieken",
		"stats.total_movies":   "Films gezien",
		"stats.total_episodes": "Afleveringen gezien",
		"stats.total_time":     "Totale kijktijd",
		"stats.avg_rating":     "Gemiddelde beoordeling",
		"stats.genres":         "Genres",
		"stats.rating_dist":    "Beoordelingsverdeling",
		"stats.monthly":        "Maandelijkse activiteit",
		"stats.streak":         "Huidige reeks",
		"stats.longest_streak": "Langste reeks",
		"stats.days":           "dagen",

		// Settings (new sections)
		"settings.webhooks":       "Webhooks",
		"settings.webhooks_empty": "Geen webhooks geconfigureerd.",
		"settings.webhook_url":    "Webhook URL",
		"settings.ics":            "ICS Kalender",
		"settings.ics_hint":       "Abonneer op je kalender in elke app die iCalendar-feeds ondersteunt.",
		"settings.ics_regenerate": "Token opnieuw genereren",
		"settings.ics_none":       "Nog geen ICS-token gegenereerd.",
		"settings.trakt":          "Trakt Import",
		"settings.trakt_hint":     "Upload een Trakt JSON-export. Films worden als gezien gemarkeerd. Series worden aan je bibliotheek toegevoegd (afleveringsvoortgang moet handmatig worden bijgehouden).",

		// Admin (extended)
		"admin.role":             "Rol",
		"admin.delete_user":      "Gebruiker verwijderen",
		"admin.actions":          "Acties",
		"admin.recalc_statuses":  "Afgeleide statussen herberekenen",
		"action.run":             "uitvoeren",
	})
}
