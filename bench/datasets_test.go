package bench

import (
	stdjson "encoding/json"
	"strconv"
	"strings"
)

type Small struct {
	Name  string  `json:"name"`
	Age   int     `json:"age"`
	Email string  `json:"email"`
	Score float64 `json:"score"`
}

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
	State  string `json:"state"`
	Zip    string `json:"zip"`
}

type Medium struct {
	ID      uint64   `json:"id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Age     int      `json:"age"`
	Score   float64  `json:"score"`
	Active  bool     `json:"active"`
	Tags    []string `json:"tags"`
	Address Address  `json:"address"`
}

type Large struct {
	Users []Medium `json:"users"`
	Total int      `json:"total"`
	Page  int      `json:"page"`
}

type ShortStrings struct {
	Words []string `json:"words"`
}

type LongString struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type EscapedStrings struct {
	Lines []string `json:"lines"`
}

type Ints struct {
	Values []int64 `json:"values"`
}

type Floats struct {
	Values []float64 `json:"values"`
}

type TwitterUser struct {
	ID              int64  `json:"id"`
	IDStr           string `json:"id_str"`
	Name            string `json:"name"`
	ScreenName      string `json:"screen_name"`
	Location        string `json:"location"`
	Description     string `json:"description"`
	URL             string `json:"url,omitempty"`
	FollowersCount  int    `json:"followers_count"`
	FriendsCount    int    `json:"friends_count"`
	Verified        bool   `json:"verified"`
	ProfileImageURL string `json:"profile_image_url"`
}

type Hashtag struct {
	Text    string `json:"text"`
	Indices []int  `json:"indices"`
}

type URLEntity struct {
	URL         string `json:"url"`
	ExpandedURL string `json:"expanded_url"`
	Indices     []int  `json:"indices"`
}

type Mention struct {
	ID         int64  `json:"id"`
	ScreenName string `json:"screen_name"`
	Indices    []int  `json:"indices"`
}

type Entities struct {
	Hashtags     []Hashtag   `json:"hashtags"`
	URLs         []URLEntity `json:"urls"`
	UserMentions []Mention   `json:"user_mentions"`
}

type Coordinates struct {
	Type        string     `json:"type"`
	Coordinates [2]float64 `json:"coordinates"`
}

type Status struct {
	ID                int64             `json:"id"`
	IDStr             string            `json:"id_str"`
	CreatedAt         string            `json:"created_at"`
	Text              string            `json:"text"`
	Source            string            `json:"source"`
	Truncated         bool              `json:"truncated"`
	InReplyToStatusID *int64            `json:"in_reply_to_status_id,omitempty"`
	User              TwitterUser       `json:"user"`
	Coordinates       *Coordinates      `json:"coordinates"`
	Entities          Entities          `json:"entities"`
	RetweetCount      int               `json:"retweet_count"`
	FavoriteCount     int               `json:"favorite_count"`
	Favorited         bool              `json:"favorited"`
	Retweeted         bool              `json:"retweeted"`
	Lang              string            `json:"lang"`
	Metadata          map[string]string `json:"metadata"`
}

type Twitter struct {
	Statuses []Status `json:"statuses"`
}

type Node struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Weight   float64 `json:"weight"`
	Children []*Node `json:"children"`
}

type dataset struct {
	name      string
	value     any
	newTarget func() any
	json      []byte
}

func makeMedium(i int) Medium {
	return Medium{
		ID:     uint64(i + 1),
		Name:   "User " + strconv.Itoa(i),
		Email:  "user" + strconv.Itoa(i) + "@example.com",
		Age:    20 + i%50,
		Score:  float64(50+i%50) + float64(i%10)/10,
		Active: i%2 == 0,
		Tags:   []string{"tag1", "tag2", "role-" + strconv.Itoa(i%7)},
		Address: Address{
			Street: strconv.Itoa(i) + " Elm St",
			City:   "Springfield",
			State:  "IL",
			Zip:    "62701",
		},
	}
}

func makeLarge(n int) *Large {
	users := make([]Medium, n)
	for i := range users {
		users[i] = makeMedium(i)
	}
	return &Large{Users: users, Total: n, Page: 1}
}

var wordList = []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliett", "kilo", "lima", "mike", "november", "oscar", "papa"}

func makeShortStrings(n int) *ShortStrings {
	w := make([]string, n)
	for i := range w {
		w[i] = wordList[i%len(wordList)] + strconv.Itoa(i%100)
	}
	return &ShortStrings{Words: w}
}

func makeLongString(n int) *LongString {
	var sb strings.Builder
	sb.Grow(n)
	for i := 0; sb.Len() < n; i++ {
		sb.WriteString(wordList[i%len(wordList)])
		sb.WriteByte(' ')
	}
	return &LongString{Title: "A long document", Body: sb.String()[:n]}
}

var escapedFragments = []string{
	`He said "hello" and left.`,
	"tab\tseparated\tvalues",
	"line one\nline two\r\n",
	`C:\Users\path\to\file`,
	"<script>alert('x')</script> & more",
	"café naïve résumé",
	"日本語のテキストです",
	"emoji 😀🎉 party",
	"控制字符\u0001\u001f",
}

func makeEscapedStrings(n int) *EscapedStrings {
	lines := make([]string, n)
	for i := range lines {
		var sb strings.Builder
		for j := range 6 {
			sb.WriteString(escapedFragments[(i+j)%len(escapedFragments)])
			sb.WriteByte(' ')
		}
		lines[i] = sb.String()
	}
	return &EscapedStrings{Lines: lines}
}

func makeInts(n int) *Ints {
	v := make([]int64, n)
	for i := range v {
		switch i % 4 {
		case 0:
			v[i] = int64(i % 10)
		case 1:
			v[i] = int64(i * 7919)
		case 2:
			v[i] = -int64(i) * 104729
		default:
			v[i] = int64(i) * 1_000_000_007
		}
	}
	return &Ints{Values: v}
}

func makeFloats(n int) *Floats {
	v := make([]float64, n)
	for i := range v {
		v[i] = -73.0 + float64(i%997)*0.0123456789 + float64(i%13)/1e7
	}
	return &Floats{Values: v}
}

var tweetTexts = []string{
	"Just shipped a new release 🚀 check it out https://example.com/release",
	"@alice thanks for the review! #golang #performance",
	"東京は今日とても暑いです。皆さん気をつけて！",
	"Benchmarks show a \"significant\" improvement over the baseline",
	"RT @bob: JSON parsing is the bottleneck in 90% of services",
}

func makeTwitter(n int) *Twitter {
	ss := make([]Status, n)
	for i := range ss {
		id := int64(505874924095815681 + i)
		var reply *int64
		if i%3 == 0 {
			r := id - 1000
			reply = &r
		}
		var coords *Coordinates
		if i%4 == 0 {
			coords = &Coordinates{Type: "Point", Coordinates: [2]float64{139.6917 + float64(i)/1000, 35.6895 - float64(i)/1000}}
		}
		ss[i] = Status{
			ID:                id,
			IDStr:             strconv.FormatInt(id, 10),
			CreatedAt:         "Sun Aug 31 00:29:15 +0000 2014",
			Text:              tweetTexts[i%len(tweetTexts)],
			Source:            `<a href="http://twitter.com/download/iphone" rel="nofollow">Twitter for iPhone</a>`,
			Truncated:         false,
			InReplyToStatusID: reply,
			User: TwitterUser{
				ID:              int64(1186275104 + i),
				IDStr:           strconv.Itoa(1186275104 + i),
				Name:            "user name " + strconv.Itoa(i),
				ScreenName:      "screen_" + strconv.Itoa(i),
				Location:        "Tokyo, Japan",
				Description:     "Software engineer. Opinions are my own. 日本語OK",
				FollowersCount:  i * 13,
				FriendsCount:    i * 7,
				Verified:        i%10 == 0,
				ProfileImageURL: "http://pbs.twimg.com/profile_images/" + strconv.Itoa(i) + "/avatar_normal.jpeg",
			},
			Coordinates: coords,
			Entities: Entities{
				Hashtags: []Hashtag{{Text: "golang", Indices: []int{10, 17}}},
				URLs:     []URLEntity{{URL: "https://t.co/abc" + strconv.Itoa(i), ExpandedURL: "https://example.com/release/" + strconv.Itoa(i), Indices: []int{40, 63}}},
				UserMentions: []Mention{
					{ID: int64(1000 + i), ScreenName: "alice", Indices: []int{0, 6}},
				},
			},
			RetweetCount:  i % 100,
			FavoriteCount: i % 37,
			Lang:          "en",
			Metadata:      map[string]string{"iso_language_code": "en", "result_type": "recent"},
		}
	}
	return &Twitter{Statuses: ss}
}

func makeTree(depth, fanout int, next *int) *Node {
	*next++
	n := &Node{ID: *next, Name: "node-" + strconv.Itoa(*next), Weight: float64(*next) * 0.5, Children: []*Node{}}
	if depth > 0 {
		n.Children = make([]*Node, fanout)
		for i := range n.Children {
			n.Children[i] = makeTree(depth-1, fanout, next)
		}
	}
	return n
}

func makeChain(depth int) *Node {
	root := &Node{ID: 0, Name: "root"}
	cur := root
	for i := 1; i < depth; i++ {
		c := &Node{ID: i, Name: "n" + strconv.Itoa(i), Weight: float64(i), Children: []*Node{}}
		cur.Children = []*Node{c}
		cur = c
	}
	return root
}

func mustStdJSON(v any) []byte {
	b, err := stdjson.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func typed[T any](name string, v *T) dataset {
	return dataset{name: name, value: v, newTarget: func() any { return new(T) }, json: mustStdJSON(v)}
}

func buildDatasets() []dataset {
	treeCounter := 0
	return []dataset{
		typed("small", &Small{Name: "Alice", Age: 30, Email: "alice@example.com", Score: 98.5}),
		typed("medium", new(makeMedium(42))),
		typed("large_100", makeLarge(100)),
		typed("xlarge_10k", makeLarge(10_000)),
		typed("short_strings", makeShortStrings(2_000)),
		typed("long_string_64k", makeLongString(64<<10)),
		typed("long_string_1m", makeLongString(1<<20)),
		typed("escaped_strings", makeEscapedStrings(500)),
		typed("ints_10k", makeInts(10_000)),
		typed("floats_10k", makeFloats(10_000)),
		typed("twitter_100", makeTwitter(100)),
		typed("tree_3280", makeTree(7, 3, &treeCounter)),
		typed("deep_chain_200", makeChain(200)),
	}
}

var datasets = buildDatasets()

type genericDataset struct {
	name  string
	value any
	json  []byte
}

func buildGenericDatasets() []genericDataset {
	var out []genericDataset
	for _, d := range datasets {
		switch d.name {
		case "small", "medium", "large_100", "twitter_100", "floats_10k", "escaped_strings", "tree_3280":
			var v any
			if err := stdjson.Unmarshal(d.json, &v); err != nil {
				panic(err)
			}
			out = append(out, genericDataset{name: d.name, value: v, json: d.json})
		}
	}
	return out
}

var genericDatasets = buildGenericDatasets()
