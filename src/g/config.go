package g

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
	"smartping/src/static"

	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	Root            string
	Cfg             Config
	SelfCfg         NetworkMember
	AlertStatus     map[string]bool
	AlertStatusLock sync.RWMutex
	AuthUserIpMap   map[string]bool
	AuthAgentIpMap  map[string]bool
	AuthIpLock      sync.RWMutex
	ToolLimit       map[string]int
	ToolLimitLock   sync.RWMutex
	Db              *sql.DB
	DLock           sync.Mutex
	LocalTimezone   *time.Location
	HttpClient      *http.Client
)

func IsExist(fp string) bool {
	_, err := os.Stat(fp)
	return err == nil || os.IsExist(err)
}

func ReadConfig(filename string) Config {
	config := Config{}
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal("Config Not Found!")
	}
	defer file.Close()
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func GetRoot() string {
	//return "D:\\gopath\\src\\github.com\\smartping\\smartping"
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal("Get Root Path Error:", err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

func releaseDefaultFiles() {
	// Create directories if not exist
	os.MkdirAll(Root+"/conf", 0755)
	os.MkdirAll(Root+"/db", 0755)

	// Release config-base.json
	configBase := Root + "/conf/config-base.json"
	if !IsExist(configBase) {
		data, err := static.Files.ReadFile("conf/config-base.json")
		if err != nil {
			log.Fatalln("[Fault]read embedded config-base.json fail:", err)
		}
		if err := os.WriteFile(configBase, data, 0644); err != nil {
			log.Fatalln("[Fault]write config-base.json fail:", err)
		}
		log.Println("[Info]released config-base.json")
	}

	// Release database-base.db
	dbBase := Root + "/db/database-base.db"
	if !IsExist(dbBase) {
		data, err := static.Files.ReadFile("db/database-base.db")
		if err != nil {
			log.Fatalln("[Fault]read embedded database-base.db fail:", err)
		}
		if err := os.WriteFile(dbBase, data, 0644); err != nil {
			log.Fatalln("[Fault]write database-base.db fail:", err)
		}
		log.Println("[Info]released database-base.db")
	}
}

func ParseConfig(ver string) {
	Root = GetRoot()

	// Release default files if not exist
	releaseDefaultFiles()

	cfile := "config.json"
	if !IsExist(Root + "/conf/" + "config.json") {
		if !IsExist(Root + "/conf/" + "config-base.json") {
			log.Fatalln("[Fault]config file:", Root+"/conf/"+"config(-base).json", "both not existent.")
		}
		cfile = "config-base.json"
	}
	InitLogger(Root)
	Cfg = ReadConfig(Root + "/conf/" + cfile)
	if Cfg.Name == "" {
		Cfg.Name, _ = os.Hostname()
	}
	if Cfg.Addr == "" {
		Cfg.Addr = "127.0.0.1"
	}
	Cfg.Ver = ver
	if !IsExist(Root + "/db/" + "database.db") {
		if !IsExist(Root + "/db/" + "database-base.db") {
			log.Fatalln("[Fault]db file:", Root+"/db/"+"database(-base).db", "both not existent.")
		}
		src, err := os.Open(Root + "/db/" + "database-base.db")
		if err != nil {
			log.Fatalln("[Fault]db-base file open error.")
		}
		defer src.Close()
		dst, err := os.OpenFile(Root+"/db/"+"database.db", os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			log.Fatalln("[Fault]db-base file copy error.")
		}
		defer dst.Close()
		_, err = io.Copy(dst, src)
		if err != nil {
			log.Fatalln("[Fault]db-base file copy error:", err)
		}
	}
	logrus.Info("Config loaded")
	var err error
	Db, err = sql.Open("sqlite", Root+"/db/database.db")
	if err != nil {
		log.Fatalln("[Fault]db open fail .", err)
	}
	LocalTimezone = time.Local
	HttpClient = &http.Client{Timeout: 10 * time.Second}
	SelfCfg = Cfg.Network[Cfg.Addr]
	AlertStatus = map[string]bool{}
	ToolLimit = map[string]int{}
	saveAuth()
}

func SaveCloudConfig(url string) (Config, error) {
	config := Config{}
	resp, err := HttpClient.Get(url)
	if err != nil {
		return config, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	err = json.Unmarshal(body, &config)
	if err != nil {
		config.Name = string(body)
		return config, err
	}
	Name := Cfg.Name
	Addr := Cfg.Addr
	Ver := Cfg.Ver
	Password := Cfg.Password
	Port := Cfg.Port
	Endpoint := Cfg.Mode["Endpoint"]
	Cfg = config
	Cfg.Name = Name
	Cfg.Addr = Addr
	Cfg.Ver = Ver
	Cfg.Port = Port
	Cfg.Password = Password
	Cfg.Mode["LastSuccTime"] = time.Now().Format("2006-01-02 15:04:05")
	Cfg.Mode["Status"] = "true"
	Cfg.Mode["Endpoint"] = Endpoint
	Cfg.Mode["Type"] = "cloud"
	SelfCfg = Cfg.Network[Cfg.Addr]
	saveAuth()
	return config, nil
}

func SaveConfig() error {
	saveAuth()
	rrs, _ := json.Marshal(Cfg)
	var out bytes.Buffer
	errjson := json.Indent(&out, rrs, "", "\t")
	if errjson != nil {
		logrus.Error("[func:SaveConfig] Json Parse ", errjson)
		return errjson
	}
	err := os.WriteFile(Root+"/conf/"+"config.json", []byte(out.String()), 0644)
	if err != nil {
		logrus.Error("[func:SaveConfig] Config File Write", err)
		return err
	}
	return nil
}

func saveAuth() {
	AuthIpLock.Lock()
	defer AuthIpLock.Unlock()
	AuthUserIpMap = map[string]bool{}
	AuthAgentIpMap = map[string]bool{}
	for _, k := range Cfg.Network {
		AuthAgentIpMap[k.Addr] = true
	}
	Cfg.Authiplist = strings.Replace(Cfg.Authiplist, " ", "", -1)
	if Cfg.Authiplist != "" {
		authiplist := strings.Split(Cfg.Authiplist, ",")
		for _, ip := range authiplist {
			AuthUserIpMap[ip] = true
		}
	}
}
