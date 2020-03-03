package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/go-ini/ini"
	"github.com/juju/gnuflag"
	"github.com/zeebo/bencode"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Flags struct {
	delugedir, qbitdir, config, ddir, btconf, btbackup, replace string
	withoutLabels, withoutTags                                  bool
}

type Channels struct {
	comChannel     chan string
	errChannel     chan string
	boundedChannel chan bool
}

type Replace struct {
	from, to string
}

type NewTorrentStructure struct {
	ActiveTime          int64          `bencode:"active_time"`
	AddedTime           int64          `bencode:"added_time"`
	AnnounceToDht       int64          `bencode:"announce_to_dht"`
	AnnounceToLsd       int64          `bencode:"announce_to_lsd"`
	AnnounceToTrackers  int64          `bencode:"announce_to_trackers"`
	AutoManaged         int64          `bencode:"auto_managed"`
	BannedPeers         string         `bencode:"banned_peers"`
	BannedPeers6        string         `bencode:"banned_peers6"`
	BlockPerPiece       int64          `bencode:"blocks per piece"`
	CompletedTime       int64          `bencode:"completed_time"`
	DownloadRateLimit   int64          `bencode:"download_rate_limit"`
	FileSizes           [][]int64      `bencode:"file sizes"`
	FileFormat          string         `bencode:"file-format"`
	FileVersion         int64          `bencode:"file-version"`
	FilePriority        []int          `bencode:"file_priority"`
	FinishedTime        int64          `bencode:"finished_time"`
	InfoHash            string         `bencode:"info-hash"`
	LastSeenComplete    int64          `bencode:"last_seen_complete"`
	LibtorrentVersion   string         `bencode:"libtorrent-version"`
	MaxConnections      int64          `bencode:"max_connections"`
	MaxUploads          int64          `bencode:"max_uploads"`
	NumDownloaded       int64          `bencode:"num_downloaded"`
	NumIncomplete       int64          `bencode:"num_incomplete"`
	MappedFiles         []string       `bencode:"mapped_files,omitempty"`
	Paused              int64          `bencode:"paused"`
	Peers               string         `bencode:"peers"`
	Peers6              string         `bencode:"peers6"`
	Pieces              []byte         `bencode:"pieces"`
	QbthasRootFolder    int64          `bencode:"qBt-hasRootFolder"`
	Qbtcategory         string         `bencode:"qBt-category,omitempty"`
	Qbtname             string         `bencode:"qBt-name"`
	QbtqueuePosition    int            `bencode:"qBt-queuePosition"`
	QbtratioLimit       int64          `bencode:"qBt-ratioLimit"`
	QbtsavePath         string         `bencode:"qBt-savePath"`
	QbtseedStatus       int64          `bencode:"qBt-seedStatus"`
	QbtseedingTimeLimit int64          `bencode:"qBt-seedingTimeLimit"`
	Qbttags             []string       `bencode:"qBt-tags"`
	QbttempPathDisabled int64          `bencode:"qBt-tempPathDisabled"`
	SavePath            string         `bencode:"save_path"`
	SeedMode            int64          `bencode:"seed_mode"`
	SeedingTime         int64          `bencode:"seeding_time"`
	SequentialDownload  int64          `bencode:"sequential_download"`
	SuperSeeding        int64          `bencode:"super_seeding"`
	TotalDownloaded     int64          `bencode:"total_downloaded"`
	TotalUploaded       int64          `bencode:"total_uploaded"`
	Trackers            [][]string     `bencode:"trackers"`
	UploadRateLimit     int64          `bencode:"upload_rate_limit"`
	Unfinished          *[]interface{} `bencode:"unfinished,omitempty"`
	hasFiles            bool
	torrentFilePath     string
	torrentFile         map[string]interface{}
	path                string
	fileSizes           int64
	sizeAndPrio         [][]int64
	torrentFileList     []string
	nPieces             int64
	pieceLenght         int64
	replace             []Replace
}

func ASCIIconvert(s string) string {
	var buffer bytes.Buffer
	for _, c := range s {
		if c > 127 {
			buffer.WriteString(`\x` + strconv.FormatUint(uint64(c), 16))
		} else {
			buffer.WriteString(string(c))
		}
	}
	return buffer.String()
}

func checknotexists(s string, tags []string) (bool, string) {
	for _, value := range tags {
		if value == s {
			return false, s
		}
	}
	return true, s
}

func decodetorrentfile(path string) (map[string]interface{}, error) {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var torrent map[string]interface{}
	if err := bencode.DecodeBytes(dat, &torrent); err != nil {
		return nil, err
	}
	return torrent, nil
}

func encodetorrentfile(path string, newstructure *NewTorrentStructure) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Create(path)
		if err != nil {
			return err
		}
	}

	file, err := os.OpenFile(path, os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	bufferedWriter := bufio.NewWriter(file)
	enc := bencode.NewEncoder(bufferedWriter)
	if err := enc.Encode(newstructure); err != nil {
		return err
	}
	bufferedWriter.Flush()
	return nil
}

func copyfile(src string, dst string) error {
	originalFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer originalFile.Close()
	newFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer newFile.Close()
	if _, err := io.Copy(newFile, originalFile); err != nil {
		return err
	}
	if err := newFile.Sync(); err != nil {
		return err
	}
	return nil
}

type Alabels struct {
	_             map[string]interface{}
	TorrentLabels map[string]string `json:"torrent_labels,omitempty"`
}

func logic(key string, newstructure NewTorrentStructure, flags *Flags, chans *Channels, torrentspath *string, position int,
	wg *sync.WaitGroup, hashlabels *Alabels) error {
	defer wg.Done()
	defer func() {
		<-chans.boundedChannel
	}()
	defer func() {
		if r := recover(); r != nil {
			chans.errChannel <- fmt.Sprintf(
				"Panic while processing torrent %v:\n======\nReason: %v.\nText panic:\n%v\n======",
				key, r, string(debug.Stack()))
		}
	}()
	var err error
	newstructure.torrentFilePath = *torrentspath + key + ".torrent"
	if _, err = os.Stat(newstructure.torrentFilePath); os.IsNotExist(err) {
		chans.errChannel <- fmt.Sprintf("Can't find torrent file %v for %v", newstructure.torrentFilePath, key)
		return err
	}
	newstructure.torrentFile, err = decodetorrentfile(newstructure.torrentFilePath)
	if err != nil {
		chans.errChannel <- fmt.Sprintf("Can't decode torrent file %v for %v", newstructure.torrentFilePath, key)
		return err
	}
	if _, ok := newstructure.torrentFile["info"].(map[string]interface{})["files"]; ok {
		newstructure.QbthasRootFolder = 1
	} else {
		newstructure.QbthasRootFolder = 0
	}
	newstructure.QbtqueuePosition = position
	newstructure.QbtqueuePosition = 1
	newstructure.QbtratioLimit = -2000
	newstructure.QbtseedStatus = 1
	newstructure.QbtseedingTimeLimit = -2
	newstructure.QbttempPathDisabled = 0
	newstructure.Qbtname = ""
	newstructure.QbthasRootFolder = 0
	if len(newstructure.replace) != 0 {
		for _, pattern := range newstructure.replace {
			newstructure.SavePath = strings.ReplaceAll(newstructure.SavePath, pattern.from, pattern.to)
		}
	}
	newstructure.QbtsavePath = newstructure.SavePath
	if flags.withoutLabels == false || flags.withoutTags == false {
		if label, ok := hashlabels.TorrentLabels[key]; ok {
			if flags.withoutLabels == false {
				newstructure.Qbtcategory = label
			} else {
				newstructure.Qbtcategory = ""
			}
			if flags.withoutTags == false {
				newstructure.Qbttags = append(newstructure.Qbttags, label)
			} else {
				newstructure.Qbttags = []string{}
			}
		}
	}
	if err = encodetorrentfile(flags.qbitdir+key+".fastresume", &newstructure); err != nil {
		chans.errChannel <- fmt.Sprintf("Can't create qBittorrent fastresume file %v", flags.qbitdir+key+".fastresume")
		return err
	}
	if err = copyfile(newstructure.torrentFilePath, flags.qbitdir+key+".torrent"); err != nil {
		chans.errChannel <- fmt.Sprintf("Can't create qBittorrent torrent file %v", flags.qbitdir+key+".torrent")
		return err
	}
	chans.comChannel <- fmt.Sprintf("Sucessfully imported %v", newstructure.torrentFile["info"].(map[string]interface{})["name"].(string))
	return nil
}

func main() {
	flags := Flags{}
	sep := string(os.PathSeparator)
	switch OS := runtime.GOOS; OS {
	case "windows":
		flags.ddir = os.Getenv("APPDATA") + sep + "deluge" + sep
		flags.btconf = os.Getenv("APPDATA") + sep + "qBittorrent" + sep + "qBittorrent.ini"
		flags.btbackup = os.Getenv("LOCALAPPDATA") + sep + "qBittorrent" + sep + "BT_backup" + sep
	case "linux":
		usr, err := user.Current()
		if err != nil {
			panic(err)
		}
		flags.ddir = usr.HomeDir + sep + ".config" + sep + "deluge" + sep
		flags.btconf = usr.HomeDir + sep + ".config" + sep + "qBittorrent" + sep + "qBittorrent.conf"
		flags.btbackup = usr.HomeDir + sep + ".local" + sep + "share" + sep + "data" + sep + "qBittorrent" + sep + "BT_backup"
	case "darwin":
		usr, err := user.Current()
		if err != nil {
			panic(err)
		}
		flags.ddir = usr.HomeDir + sep + ".config" + sep + "deluge" + sep
		flags.config = usr.HomeDir + sep + ".config" + sep + "qBittorrent" + sep + "qbittorrent.ini"
		flags.btbackup = usr.HomeDir + sep + "Library" + sep + "Application Support" + sep + "QBittorrent" + sep + "BT_backup" + sep
	}
	gnuflag.StringVar(&flags.delugedir, "source", flags.ddir,
		"Source directory that contains deluge files and state dir")
	gnuflag.StringVar(&flags.delugedir, "s", flags.ddir,
		"Source directory that contains deluge files and state dir")
	gnuflag.StringVar(&flags.qbitdir, "destination", flags.btbackup,
		"Destination directory BT_backup (as default)")
	gnuflag.StringVar(&flags.qbitdir, "d", flags.btbackup,
		"Destination directory BT_backup (as default)")
	gnuflag.StringVar(&flags.config, "qconfig", flags.btconf,
		"qBittorrent config files (for write tags)")
	gnuflag.StringVar(&flags.config, "c", flags.btconf,
		"qBittorrent config files (for write tags)")
	gnuflag.BoolVar(&flags.withoutLabels, "without-labels", false, "Do not export/import labels")
	gnuflag.BoolVar(&flags.withoutTags, "without-tags", false, "Do not export/import tags")
	gnuflag.StringVar(&flags.replace, "replace", "", "Replace paths.\n	"+
		"Delimiter for replaces - ;\n	"+
		"Delimiter for from/to - ,\n	Example: \"D:\\films,/home/user/films;\\,/\"\n	"+
		"If you use path separator different from you system, declare it mannually")
	gnuflag.Parse(true)

	if flags.replace != "" {
		for _, str := range strings.Split(flags.replace, ";") {
			patterns := strings.Split(str, ",")
			if len(patterns) < 2 {
				log.Println("Bad replace pattern")
				time.Sleep(30 * time.Second)
				os.Exit(1)
			}
		}
	}

	if string(flags.delugedir[len(flags.delugedir)-1]) != sep {
		flags.delugedir += sep
	}
	if string(flags.qbitdir[len(flags.qbitdir)-1]) != sep {
		flags.qbitdir += sep
	}

	if _, err := os.Stat(flags.delugedir); os.IsNotExist(err) {
		log.Println("Can't find deluge folder")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	if _, err := os.Stat(flags.qbitdir); os.IsNotExist(err) {
		log.Println("Can't find qBittorrent folder")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	torrentspath := flags.delugedir + "state" + sep
	if _, err := os.Stat(torrentspath); os.IsNotExist(err) {
		log.Println("Can't find deluge state directory")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	resumefilepath := flags.delugedir + "state" + sep + "torrents.fastresume"
	if _, err := os.Stat(resumefilepath); os.IsNotExist(err) {
		log.Println("Can't find deluge fastresume file")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	fastresumefile, err := decodetorrentfile(resumefilepath)
	if err != nil {
		log.Println("Can't decode deluge fastresume file")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	if flags.withoutTags == false || flags.withoutLabels == false {
		if _, err := os.Stat(flags.delugedir + "label.conf"); os.IsNotExist(err) {
			log.Println("Can't read Deluge label.conf file. Skipping")
			flags.withoutTags, flags.withoutLabels = true, true
		}
		if _, err := os.Stat(flags.config); os.IsNotExist(err) {
			fmt.Println("Can not read qBittorrent config file. Try run and close qBittorrent if you have not done" +
				" so already, or specify the path explicitly or do not import tags")
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
	}
	color.Green("It will be performed processing from directory %v to directory %v\n", flags.delugedir, flags.qbitdir)
	color.HiRed("Check that the qBittorrent is turned off and the directory %v and config %v is backed up.\n\n",
		flags.qbitdir, flags.config)
	fmt.Println("Press Enter to start")
	fmt.Scanln()
	log.Println("Started")
	totaljobs := len(fastresumefile)
	numjob := 1
	var wg sync.WaitGroup
	chans := Channels{comChannel: make(chan string, totaljobs),
		errChannel:     make(chan string, totaljobs),
		boundedChannel: make(chan bool, runtime.GOMAXPROCS(0)*2)}

	positionnum := 0
	var jsn bytes.Buffer
	var hashlabels Alabels
	var oldtags string
	var newtags []string
	if flags.withoutTags == false || flags.withoutLabels == false {
		if jsons, err := ioutil.ReadFile(flags.delugedir + "label.conf"); err != nil {
			log.Fatal(err)
		} else {
			toggle := false
			for _, char := range jsons {
				if toggle {
					jsn.WriteString(string(char))
				}
				if toggle == false && string(char) == "}" {
					toggle = true
				}
			}
		}
		if err := json.Unmarshal(jsn.Bytes(), &hashlabels); err != nil {
			flags.withoutTags, flags.withoutLabels = true, true
		}
	}
	for key, value := range fastresumefile {
		positionnum++
		var decodedval NewTorrentStructure
		if err := bencode.DecodeString(value.(string), &decodedval); err != nil {
			torrentfile := map[string]interface{}{}
			torrentfilepath := flags.delugedir + "state" + sep + key + ".torrent"
			if _, err = os.Stat(torrentfilepath); os.IsNotExist(err) {
				chans.errChannel <- fmt.Sprintf("Can't find torrent file %v. Can't decode string %v. Continue", torrentfilepath, key)
				continue
			}
			torrentfile, err = decodetorrentfile(torrentfilepath)
			if err != nil {
				chans.errChannel <- fmt.Sprintf("Can't decode torrent file %v. Can't decode string %v. Continue", torrentfilepath, key)
				continue
			}
			torrentname := torrentfile["info"].(map[string]interface{})["name"].(string)
			log.Printf("Can't decode row %v with torrent %v. Continue", key, torrentname)
		}
		wg.Add(1)
		chans.boundedChannel <- true
		go logic(key, decodedval, &flags, &chans, &torrentspath, positionnum, &wg, &hashlabels)
	}

	go func() {
		wg.Wait()
		close(chans.comChannel)
		close(chans.errChannel)
	}()
	for message := range chans.comChannel {
		fmt.Printf("%v/%v %v \n", numjob, totaljobs, message)
		numjob++
	}

	var waserrors bool
	for message := range chans.errChannel {
		log.Printf("%v \n", message)
		waserrors = true
	}

	if flags.withoutTags == false {
		for _, label := range hashlabels.TorrentLabels {
			if len(label) > 0 {
				if ok, tag := checknotexists(ASCIIconvert(label), newtags); ok {
					newtags = append(newtags, tag)
				}
			}
		}
		cfg, err := ini.Load(flags.config)
		ini.PrettyFormat = false
		ini.PrettySection = false
		if err != nil {
			fmt.Println("Can not read qBittorrent config file. Try to specify the path explicitly or do not import tags")
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
		if _, err := cfg.GetSection("BitTorrent"); err != nil {
			cfg.NewSection("BitTorrent")

			//Dirty hack for section order. Sorry
			kv := cfg.Section("Network").KeysHash()
			cfg.DeleteSection("Network")
			cfg.NewSection("Network")
			for key, value := range kv {
				cfg.Section("Network").NewKey(key, value)
			}
			//End of dirty hack
		}
		if cfg.Section("BitTorrent").HasKey("Session\\Tags") {
			oldtags = cfg.Section("BitTorrent").Key("Session\\Tags").String()
			for _, tag := range strings.Split(oldtags, ", ") {
				if ok, t := checknotexists(tag, newtags); ok {
					newtags = append(newtags, t)
				}
			}
			cfg.Section("BitTorrent").Key("Session\\Tags").SetValue(strings.Join(newtags, ", "))
		} else {
			cfg.Section("BitTorrent").NewKey("Session\\Tags", strings.Join(newtags, ", "))
		}
		cfg.SaveTo(flags.config)
	}
	fmt.Println()
	log.Println("Ended")
	if waserrors {
		log.Println("Not all torrents was processed")
	}
	fmt.Println("\nPress Enter to exit")
	fmt.Scanln()
}
