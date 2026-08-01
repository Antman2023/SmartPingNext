package g

type Config struct {
	Ver        string                         `json:"Ver"`
	Port       int                            `json:"Port"`
	Name       string                         `json:"Name"`
	Addr       string                         `json:"Addr"`
	Mode       map[string]string              `json:"Mode"`
	Base       map[string]int                 `json:"Base"`
	Topology   map[string]string              `json:"Topology"`
	Network    map[string]NetworkMember       `json:"Network"`
	Chinamap   map[string]map[string][]string `json:"Chinamap"`
	Toollimit  int                            `json:"Toollimit"`
	Authiplist string                         `json:"Authiplist"`
	Password   string                         `json:"Password"`
}

type NetworkMember struct {
	Name      string              `json:"Name"`
	Addr      string              `json:"Addr"`
	Smartping bool                `json:"Smartping"`
	Ping      []string            `json:"Ping"`
	Topology  []map[string]string `json:"Topology"`
}

// Ping Struct
type PingSt struct {
	SendPk   int
	RevcPk   int
	LossPk   int
	MinDelay float64
	AvgDelay float64
	MaxDelay float64
}

type PingLog struct {
	Logtime  string
	Maxdelay string
	Mindelay string
	Avgdelay string
	Losspk   string
}

type AlertLog struct {
	Logtime    string
	Targetip   string
	Targetname string
	Tracert    string
	Fromip     string
	Fromname   string
}

type ChinaMp struct {
	Text     string              `json:"text"`
	Subtext  string              `json:"subtext"`
	Avgdelay map[string][]MapVal `json:"avgdelay"`
}

type MapVal struct {
	Value float64 `json:"value"`
	Name  string  `json:"name"`
}

type ToolsRes struct {
	Status string `json:"status"`
	Error  string `json:"error"`
	Ip     string `json:"ip"`
	Ping   PingSt `json:"ping"`
}
