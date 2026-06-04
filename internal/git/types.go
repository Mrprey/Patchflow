package git

type Remote struct {
	Name string
	URL  string
}

type Commit struct {
	SHA       string
	ShortSHA   string
	Author    string
	Date      string
	Title     string
	PRNumber  int
	PRTitle   string
	PRURL     string
	Labels    []string
	Selected  bool
	Status    string
}
