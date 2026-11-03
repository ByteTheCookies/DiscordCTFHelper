package ctftime

type Event struct {
	Organizers []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"organizers"`
	CtftimeURL string  `json:"ctftime_url"`
	CtfID      int     `json:"ctf_id"`
	Weight     float64 `json:"weight"`
	Duration   struct {
		Hours int `json:"hours"`
		Days  int `json:"days"`
	} `json:"duration"`
	LiveFeed      string `json:"live_feed"`
	Logo          string `json:"logo"`
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Start         string `json:"start"`
	Participants  int    `json:"participants"`
	Location      string `json:"location"`
	Finish        string `json:"finish"`
	Description   string `json:"description"`
	Format        string `json:"format"`
	IsVotableNow  bool   `json:"is_votable_now"`
	Prizes        string `json:"prizes"`
	FormatID      int    `json:"format_id"`
	Onsite        bool   `json:"onsite"`
	Restrictions  string `json:"restrictions"`
	URL           string `json:"url"`
	PublicVotable bool   `json:"public_votable"`
}
