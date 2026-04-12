package i18n

func init() {
	Register(LangEN, map[string]string{
		// Nav
		"nav.library":     "Library",
		"nav.calendar":    "Calendar",
		"nav.discover":    "Discover",
		"nav.suggestions": "Suggestions",
		"nav.search":      "Search",
		"nav.settings":    "Settings",
		"nav.stats":       "Statistics",
		"nav.logout":      "Logout",

		// Actions
		"action.save":    "Save",
		"action.cancel":  "Cancel",
		"action.create":  "Create",
		"action.change":  "Change",
		"action.edit":    "edit",
		"action.delete":  "delete",
		"action.add":     "Add",
		"action.remove":  "Remove",
		"action.dismiss": "Dismiss",
		"action.search":  "Search",
		"action.confirm": "confirm?",

		// Media
		"media.movie":         "Movie",
		"media.tv":            "TV Show",
		"media.plan_to_watch": "Plan to Watch",
		"media.watching":      "Watching",
		"media.watched":       "Watched",
		"media.on_hold":       "On Hold",
		"media.dropped":       "Dropped",
		"media.add_to_lib":    "Add to Library",
		"media.in_library":    "Already in Library",
		"media.mark_watched":  "Mark Watched",
		"media.mark_next":     "Mark Next Episode",
		"media.rate":          "Rate",
		"media.where_to_watch": "Where to Watch",

		// Library
		"library.title":        "Library",
		"library.empty":        "Your library is empty. Search for something to add.",
		"library.empty_filter": "No items match this filter.",
		"library.all":          "All",
		"library.sort":         "Sort",
		"library.sort_title":   "Title",
		"library.sort_added":   "Date Added",
		"library.sort_release": "Release Date",
		"library.sort_rating":  "Rating",

		// Calendar
		"calendar.title":    "Calendar",
		"calendar.upcoming": "Coming Up",
		"calendar.recent":   "Recently Released",
		"calendar.empty":    "Nothing coming up.",

		// Discover
		"discover.title":           "Discover",
		"discover.trending":        "Trending",
		"discover.popular":         "Popular",
		"discover.recommendations": "Recommendations",
		"discover.load_more":       "Load More",

		// Search
		"search.title":   "Search",
		"search.hint":    "Search movies and TV shows...",
		"search.empty":   "No results found.",

		// Tags
		"tags.title": "Tags",
		"tags.empty": "No tags yet.",
		"tags.add":   "Add Tag",

		// Keyword Watches
		"keywords.title":       "Keyword Watches",
		"keywords.empty":       "No keyword watches yet.",
		"keywords.suggestions": "Suggestions",
		"keywords.dismiss_all": "Dismiss All",

		// Settings
		"settings.title":            "Settings",
		"settings.appearance":       "Appearance",
		"settings.theme":            "Theme",
		"settings.language":         "Language",
		"settings.region":           "Region",
		"settings.region_hint":      "ISO 3166-1 code for watch providers (e.g. NL, US)",
		"settings.change_password":  "Change Password",
		"settings.current_password": "Current password",
		"settings.new_password":     "New password",

		// Auth
		"auth.login":          "Log in",
		"auth.register":       "Register",
		"auth.signin":         "Sign in to your account",
		"auth.create_account": "Create a new account",
		"auth.username":       "Username",
		"auth.password":       "Password",
		"auth.first_time":     "First time?",
		"auth.create_link":    "Create an account",
		"auth.have_account":   "Already have an account?",
		"auth.login_link":     "Log in",

		// Logout
		"logout.title":   "Log out",
		"logout.confirm": "Are you sure you want to log out?",
		"logout.yes":     "Yes, log out",

		// Admin
		"nav.admin":         "Admin",
		"admin.users_title": "Users",
		"admin.stats_title": "Statistics",

		// Stats
		"stats.title":          "Statistics",
		"stats.total_movies":   "Movies Watched",
		"stats.total_episodes": "Episodes Watched",
		"stats.total_time":     "Total Watch Time",
		"stats.avg_rating":     "Average Rating",
		"stats.genres":         "Genres",
		"stats.rating_dist":    "Rating Distribution",
		"stats.monthly":        "Monthly Activity",
		"stats.streak":         "Current Streak",
		"stats.longest_streak": "Longest Streak",
		"stats.days":           "days",

		// Settings (new sections)
		"settings.webhooks":       "Webhooks",
		"settings.webhooks_empty": "No webhooks configured.",
		"settings.webhook_url":    "Webhook URL",
		"settings.ics":            "ICS Calendar",
		"settings.ics_hint":       "Subscribe to your calendar in any app that supports iCalendar feeds.",
		"settings.ics_regenerate": "Regenerate Token",
		"settings.ics_none":       "No ICS token generated yet.",
		"settings.trakt":          "Trakt Import",
		"settings.trakt_hint":     "Upload a Trakt JSON export. Movies are marked as watched. TV shows are added to your library (episode progress must be tracked manually).",

		// Admin (extended)
		"admin.role":        "Role",
		"admin.delete_user": "Delete User",
	})
}
