package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/go-ini/ini"
	"github.com/zeebo/bencode"
	"io"
	"io/ioutil"
	"launchpad.net/gnuflag"
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
	if err := bencode.DecodeBytes([]byte(dat), &torrent); err != nil {
		return nil, err
	}
	return torrent, nil
}

func encodetorrentfile(path string, newstructure *NewTorrentStructure) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Create(path)
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

type NewTorrentStructure struct {
	Active_time          int64          `bencode:"active_time"`
	Added_time           int64          `bencode:"added_time"`
	Announce_to_dht      int64          `bencode:"announce_to_dht"`
	Announce_to_lsd      int64          `bencode:"announce_to_lsd"`
	Announce_to_trackers int64          `bencode:"announce_to_trackers"`
	Auto_managed         int64          `bencode:"auto_managed"`
	Banned_peers         string         `bencode:"banned_peers"`
	Banned_peers6        string         `bencode:"banned_peers6"`
	Blockperpiece        int64          `bencode:"blocks per piece"`
	Completed_time       int64          `bencode:"completed_time"`
	Download_rate_limit  int64          `bencode:"download_rate_limit"`
	Filesizes            [][]int64      `bencode:"file sizes"`
	Fileformat           string         `bencode:"file-format"`
	Fileversion          int64          `bencode:"file-version"`
	File_priority        []int          `bencode:"file_priority"`
	Finished_time        int64          `bencode:"finished_time"`
	Infohash             string         `bencode:"info-hash"`
	Last_seen_complete   int64          `bencode:"last_seen_complete"`
	Libtorrentversion    string         `bencode:"libtorrent-version"`
	Max_connections      int64          `bencode:"max_connections"`
	Max_uploads          int64          `bencode:"max_uploads"`
	Num_downloaded       int64          `bencode:"num_downloaded"`
	Num_incomplete       int64          `bencode:"num_incomplete"`
	Mapped_files         []string       `bencode:"mapped_files,omitempty"`
	Paused               int64          `bencode:"paused"`
	Peers                string         `bencode:"peers"`
	Peers6               string         `bencode:"peers6"`
	Pieces               []byte         `bencode:"pieces"`
	QbthasRootFolder     int64          `bencode:"qBt-hasRootFolder"`
	Qbtcategory          string         `bencode:"qBt-category,omitempty"`
	Qbtname              string         `bencode:"qBt-name"`
	QbtqueuePosition     int            `bencode:"qBt-queuePosition"`
	QbtratioLimit        int64          `bencode:"qBt-ratioLimit"`
	QbtsavePath          string         `bencode:"qBt-savePath"`
	QbtseedStatus        int64          `bencode:"qBt-seedStatus"`
	QbtseedingTimeLimit  int64          `bencode:"qBt-seedingTimeLimit"`
	Qbttags              []string       `bencode:"qBt-tags"`
	QbttempPathDisabled  int64          `bencode:"qBt-tempPathDisabled"`
	Save_path            string         `bencode:"save_path"`
	Seed_mode            int64          `bencode:"seed_mode"`
	Seeding_time         int64          `bencode:"seeding_time"`
	Sequential_download  int64          `bencode:"sequential_download"`
	Super_seeding        int64          `bencode:"super_seeding"`
	Total_downloaded     int64          `bencode:"total_downloaded"`
	Total_uploaded       int64          `bencode:"total_uploaded"`
	Trackers             [][]string     `bencode:"trackers"`
	Upload_rate_limit    int64          `bencode:"upload_rate_limit"`
	Unfinished           *[]interface{} `bencode:"unfinished,omitempty"`
	hasfiles             bool
	torrentfilepath      string
	torrentfile          map[string]interface{}
	path                 string
	filesizes            int64
	sizeandprio          [][]int64
	torrentfilelist      []string
	npieces              int64
	piecelenght          int64
}

type Alabels struct {
	_              map[string]interface{}
	Torrent_labels map[string]string `json:"torrent_labels,omitempty"`
}

func logic(key string, newstructure NewTorrentStructure, torrentspath *string, with_label *bool, with_tags *bool,
	qbitdir *string, comChannel chan string, errChannel chan string, position int, wg *sync.WaitGroup, hashlabels *Alabels) error {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			errChannel <- fmt.Sprintf(
				"Panic while processing torrent %v:\n======\nReason: %v.\nText panic:\n%v\n======",
				key, r, string(debug.Stack()))
		}
	}()
	var err error
	newstructure.torrentfilepath = *torrentspath + key + ".torrent"
	if _, err = os.Stat(newstructure.torrentfilepath); os.IsNotExist(err) {
		errChannel <- fmt.Sprintf("Can't find torrent file %v for %v", newstructure.torrentfilepath, key)
		return err
	}
	newstructure.torrentfile, err = decodetorrentfile(newstructure.torrentfilepath)
	if err != nil {
		errChannel <- fmt.Sprintf("Can't decode torrent file %v for %v", newstructure.torrentfilepath, key)
		return err
	}
	if _, ok := newstructure.torrentfile["info"].(map[string]interface{})["files"]; ok {
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
	newstructure.QbtsavePath = newstructure.Save_path
	if *with_label || *with_tags {
		if label, ok := hashlabels.Torrent_labels[key]; ok {
			if *with_label {
				newstructure.Qbtcategory = label
			} else {
				newstructure.Qbtcategory = ""
			}
			if *with_tags {
				newstructure.Qbttags = append(newstructure.Qbttags, label)
			} else {
				newstructure.Qbttags = []string{}
			}
		}
	}
	if err = encodetorrentfile(*qbitdir+key+".fastresume", &newstructure); err != nil {
		errChannel <- fmt.Sprintf("Can't create qBittorrent fastresume file %v", *qbitdir+key+".fastresume")
		return err
	}
	if err = copyfile(newstructure.torrentfilepath, *qbitdir+key+".torrent"); err != nil {
		errChannel <- fmt.Sprintf("Can't create qBittorrent torrent file %v", *qbitdir+key+".torrent")
		return err
	}
	comChannel <- fmt.Sprintf("Sucessfully imported %v", newstructure.torrentfile["info"].(map[string]interface{})["name"].(string))
	return nil
}

func main() {
	var delugedir, qbitdir, config, ddir, btconf, btbackup string
	var with_label, with_tags = true, true
	var without_label, without_tags bool
	sep := string(os.PathSeparator)
	if runtime.GOOS == "windows" {
		ddir = os.Getenv("APPDATA") + sep + "deluge" + sep
		btconf = os.Getenv("APPDATA") + sep + "qBittorrent" + sep + "qBittorrent.ini"
		btbackup = os.Getenv("LOCALAPPDATA") + sep + "qBittorrent" + sep + "BT_backup" + sep
	} else {
		usr, err := user.Current()
		if err != nil {
			panic(err)
		}
		ddir = usr.HomeDir + sep + ".config" + sep + "deluge" + sep
		btconf = usr.HomeDir + sep + ".config" + sep + "qBittorrent" + sep + "qBittorrent.conf"
		btbackup = usr.HomeDir + sep + ".local" + sep + "share" + sep + "data" + sep + "qBittorrent" + sep + "BT_backup"
	}
	gnuflag.StringVar(&delugedir, "source", ddir,
		"Source directory that contains resume.dat and torrents files")
	gnuflag.StringVar(&delugedir, "s", ddir,
		"Source directory that contains resume.dat and torrents files")
	gnuflag.StringVar(&qbitdir, "destination", btbackup,
		"Destination directory BT_backup (as default)")
	gnuflag.StringVar(&qbitdir, "d", btbackup,
		"Destination directory BT_backup (as default)")
	gnuflag.StringVar(&config, "qconfig", btconf,
		"qBittorrent config files (for write tags)")
	gnuflag.StringVar(&config, "c", btconf,
		"qBittorrent config files (for write tags)")
	gnuflag.BoolVar(&without_label, "without-labels", false, "Do not export/import labels")
	gnuflag.BoolVar(&without_tags, "without-tags", false, "Do not export/import tags")
	gnuflag.Parse(true)

	if without_label {
		with_label = false
	}
	if without_tags {
		with_tags = false
	}

	if string(delugedir[len(delugedir)-1]) != sep {
		delugedir += sep
	}
	if string(qbitdir[len(qbitdir)-1]) != sep {
		qbitdir += sep
	}

	if _, err := os.Stat(delugedir); os.IsNotExist(err) {
		log.Println("Can't find deluge folder")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	if _, err := os.Stat(qbitdir); os.IsNotExist(err) {
		log.Println("Can't find qBittorrent folder")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	torrentspath := delugedir + "state" + sep
	if _, err := os.Stat(torrentspath); os.IsNotExist(err) {
		log.Println("Can't find deluge state directory")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	resumefilepath := delugedir + "state" + sep + "torrents.fastresume"
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
	if with_tags == true || with_label {
		if _, err := os.Stat(delugedir + "label.conf"); os.IsNotExist(err) {
			log.Println("Can't read Deluge label.conf file. Skipping")
			with_label = false
			with_tags = false
		}
		if _, err := os.Stat(config); os.IsNotExist(err) {
			fmt.Println("Can not read qBittorrent config file. Try run and close qBittorrent if you have not done" +
				" so already, or specify the path explicitly or do not import tags")
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
	}
	color.Green("It will be performed processing from directory %v to directory %v\n", delugedir, qbitdir)
	color.HiRed("Check that the qBittorrent is turned off and the directory %v and config %v is backed up.\n\n",
		qbitdir, config)
	fmt.Println("Press Enter to start")
	fmt.Scanln()
	log.Println("Started")
	totaljobs := len(fastresumefile)
	numjob := 1
	var wg sync.WaitGroup
	comChannel := make(chan string, totaljobs)
	errChannel := make(chan string, totaljobs*2)
	positionnum := 0
	var jsn bytes.Buffer
	var hashlabels Alabels
	var oldtags string
	var newtags []string
	if with_tags || with_label {
		if jsons, err := ioutil.ReadFile(delugedir + "label.conf"); err != nil {
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
			with_label, with_tags = false, false
		}
	}
	for key, value := range fastresumefile {
		positionnum++
		var decodedval NewTorrentStructure
		if err := bencode.DecodeString(value.(string), &decodedval); err != nil {
			torrentfile := map[string]interface{}{}
			torrentfilepath := delugedir + "state" + sep + key + ".torrent"
			if _, err = os.Stat(torrentfilepath); os.IsNotExist(err) {
				errChannel <- fmt.Sprintf("Can't find torrent file %v. Can't decode string %v. Continue", torrentfilepath, key)
				continue
			}
			torrentfile, err = decodetorrentfile(torrentfilepath)
			if err != nil {
				errChannel <- fmt.Sprintf("Can't decode torrent file %v. Can't decode string %v. Continue", torrentfilepath, key)
				continue
			}
			torrentname := torrentfile["info"].(map[string]interface{})["name"].(string)
			log.Printf("Can't decode row %v with torrent %v. Continue", key, torrentname)
		}
		wg.Add(1)
		go logic(key, decodedval, &torrentspath, &with_label, &with_tags, &qbitdir, comChannel,
			errChannel, positionnum, &wg, &hashlabels)
	}
	go func() {
		wg.Wait()
		close(comChannel)
		close(errChannel)
	}()
	for message := range comChannel {
		fmt.Printf("%v/%v %v \n", numjob, totaljobs, message)
		numjob++
	}
	var waserrors bool
	for message := range errChannel {
		log.Printf("%v \n", message)
		waserrors = true
	}

	if with_tags == true {
		for _, label := range hashlabels.Torrent_labels {
			if len(label) > 0 {
				if ok, tag := checknotexists(ASCIIconvert(label), newtags); ok {
					newtags = append(newtags, tag)
				}
			}
		}
		cfg, err := ini.Load(config)
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
		cfg.SaveTo(config)
	}
	fmt.Println()
	log.Println("Ended")
	if waserrors {
		log.Println("Not all torrents was processed")
	}
	fmt.Println("\nPress Enter to exit")
	fmt.Scanln()
}
