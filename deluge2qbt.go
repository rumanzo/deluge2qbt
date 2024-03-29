package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
	"github.com/rumanzo/bt2qbt/pkg/fileHelpers"
	"github.com/rumanzo/bt2qbt/pkg/helpers"
	"github.com/rumanzo/bt2qbt/pkg/qBittorrentStructures"
	"github.com/rumanzo/bt2qbt/pkg/torrentStructures"
	"github.com/zeebo/bencode"
	"log"
	"os"
	"os/user"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var version, commit, date, buildImage string

type Opts struct {
	DelugeDir   string   `short:"s" long:"source" description:"Source directory that contains deluge files"`
	QBitDir     string   `short:"d" long:"destination" description:"Destination directory BT_backup (as default)"`
	WithoutTags bool     `long:"without-tags" description:"Do not export/import tags"`
	Replaces    []string `short:"r" long:"replace" description:"Replace save paths. Important: you have to use single slashes in paths\n	Delimiter for from/to is comma - ,\n	Example: -r \"D:/films,/home/user/films\" -r \"D:/music,/home/user/music\"\n"`
	Version     bool     `short:"v" long:"version" description:"Show version"`
}

type Channels struct {
	comChannel     chan string
	errChannel     chan string
	boundedChannel chan bool
}

type Replace struct {
	From, To string
}

type Alabels struct {
	_             map[string]interface{}
	TorrentLabels map[string]string `json:"torrent_labels,omitempty"`
}

func logic(fastResumeHashId string, fastResume qBittorrentStructures.QBittorrentFastresume, opts *Opts, chans *Channels, torrentsPath *string,
	wg *sync.WaitGroup, labels *Alabels, replace []*Replace) error {
	defer wg.Done()
	defer func() {
		<-chans.boundedChannel
	}()
	defer func() {
		if r := recover(); r != nil {
			chans.errChannel <- fmt.Sprintf(
				"Panic while processing torrent %v:\n======\nReason: %v.\nText panic:\n%v\n======",
				fastResumeHashId, r, string(debug.Stack()))
		}
	}()
	var err error
	torrentFilePath := *torrentsPath + fastResumeHashId + ".torrent"
	if _, err = os.Stat(torrentFilePath); os.IsNotExist(err) {
		chans.errChannel <- fmt.Sprintf("Can't find torrent file %v for %v", torrentFilePath, fastResumeHashId)
		return err
	}
	var torrentFile torrentStructures.Torrent
	err = helpers.DecodeTorrentFile(torrentFilePath, &torrentFile)
	if err != nil {
		chans.errChannel <- fmt.Sprintf("Can't decode torrent file %v for %v", torrentFilePath, fastResumeHashId)
		return err
	}

	var torrentName string
	if torrentFile.Info.NameUTF8 != "" {
		torrentName = torrentFile.Info.NameUTF8
	} else {
		torrentName = torrentFile.Info.Name
	}

	fastResume.QBtContentLayout = "Original"
	fastResume.QbtRatioLimit = -2000
	fastResume.QbtSeedStatus = 1
	fastResume.QbtSeedingTimeLimit = -2
	fastResume.QbtName = ""
	fastResume.QBtCategory = ""

	for _, pattern := range replace {
		fastResume.SavePath = strings.ReplaceAll(fastResume.SavePath, pattern.From, pattern.To)
		for mapIndex, mapPath := range fastResume.MappedFiles {
			if fileHelpers.IsAbs(mapPath) {
				fastResume.MappedFiles[mapIndex] = strings.ReplaceAll(mapPath, pattern.From, pattern.To)
			}
		}
	}

	fastResume.QbtSavePath = fastResume.SavePath

	if opts.WithoutTags == false {
		if label, ok := labels.TorrentLabels[fastResumeHashId]; ok {
			fastResume.QbtTags = append(fastResume.QbtTags, label)
		} else {
			fastResume.QbtTags = []string{}
		}
	} else {
		fastResume.QbtTags = []string{}
	}

	if err = helpers.EncodeTorrentFile(opts.QBitDir+fastResumeHashId+".fastresume", &fastResume); err != nil {
		chans.errChannel <- fmt.Sprintf("Can't create qBittorrent fastresume file %v", opts.QBitDir+fastResumeHashId+".fastresume")
		return err
	}
	if err = helpers.CopyFile(torrentFilePath, opts.QBitDir+fastResumeHashId+".torrent"); err != nil {
		chans.errChannel <- fmt.Sprintf("Can't create qBittorrent torrent file %v", opts.QBitDir+fastResumeHashId+".torrent")
		return err
	}
	chans.comChannel <- fmt.Sprintf("Sucessfully imported %v", torrentName)
	return nil
}

func main() {
	opts := Opts{}
	sep := string(os.PathSeparator)
	switch OS := runtime.GOOS; OS {
	case "windows":
		opts.DelugeDir = os.Getenv("APPDATA") + sep + "deluge" + sep
		opts.QBitDir = os.Getenv("LOCALAPPDATA") + sep + "qBittorrent" + sep + "BT_backup" + sep
	case "linux":
		usr, err := user.Current()
		if err != nil {
			panic(err)
		}
		opts.DelugeDir = usr.HomeDir + sep + ".config" + sep + "deluge" + sep
		opts.QBitDir = usr.HomeDir + sep + ".local" + sep + "share" + sep + "data" + sep + "qBittorrent" + sep + "BT_backup"
	case "darwin":
		usr, err := user.Current()
		if err != nil {
			panic(err)
		}
		opts.DelugeDir = usr.HomeDir + sep + ".config" + sep + "deluge" + sep
		opts.QBitDir = usr.HomeDir + sep + "Library" + sep + "Application Support" + sep + "QBittorrent" + sep + "BT_backup" + sep
	}
	if _, err := flags.Parse(&opts); err != nil { // https://godoc.org/github.com/jessevdk/go-flags#ErrorType
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			log.Println(err)
			time.Sleep(30 * time.Second)
			os.Exit(1)
		}
	}

	if opts.Version {
		date = time.Now().Format("2006-01-02 15:04:05")
		fmt.Printf("Version: %v\nCommit: %v\nGolang version: %v\nBuild image: %v\nBuild date: %v\n", version, commit, runtime.Version(), buildImage, date)
		os.Exit(0)
	}

	if len(opts.Replaces) != 0 {
		for _, str := range opts.Replaces {
			patterns := strings.Split(str, ",")
			if len(patterns) != 2 {
				log.Println("Bad replace pattern")
				time.Sleep(30 * time.Second)
				os.Exit(1)
			}
		}
	}

	if string(opts.DelugeDir[len(opts.DelugeDir)-1]) != sep {
		opts.DelugeDir += sep
	}
	if string(opts.QBitDir[len(opts.QBitDir)-1]) != sep {
		opts.QBitDir += sep
	}

	if _, err := os.Stat(opts.DelugeDir); os.IsNotExist(err) {
		log.Println("Can't find deluge folder")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	if _, err := os.Stat(opts.QBitDir); os.IsNotExist(err) {
		log.Println("Can't find qBittorrent folder")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	torrentsPath := opts.DelugeDir + "state" + sep
	if _, err := os.Stat(torrentsPath); os.IsNotExist(err) {
		log.Println("Can't find deluge state directory")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	resumeFilePath := opts.DelugeDir + "state" + sep + "torrents.fastresume"
	if _, err := os.Stat(resumeFilePath); os.IsNotExist(err) {
		log.Println("Can't find deluge fastresume file")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	var fastResumeFile map[string]interface{}
	err := helpers.DecodeTorrentFile(resumeFilePath, &fastResumeFile)
	if err != nil {
		log.Println("Can't decode deluge fastresume file")
		time.Sleep(30 * time.Second)
		os.Exit(1)
	}
	if opts.WithoutTags == false {
		if _, err := os.Stat(opts.DelugeDir + "label.conf"); os.IsNotExist(err) {
			log.Println("Can't read Deluge label.conf file. Skipping")
			opts.WithoutTags = true
		}
	}
	color.Green("It will be performed processing from directory %v to directory %v\n", opts.DelugeDir, opts.QBitDir)
	color.HiRed("Check that the qBittorrent is turned off and the directory %v is backed up.\n\n",
		opts.QBitDir)
	fmt.Println("Press Enter to start")
	fmt.Scanln()
	log.Println("Started")
	totaljobs := len(fastResumeFile)
	numjob := 1
	var wg sync.WaitGroup
	chans := Channels{comChannel: make(chan string, totaljobs),
		errChannel:     make(chan string, totaljobs),
		boundedChannel: make(chan bool, runtime.GOMAXPROCS(0)*2)}

	var jsn bytes.Buffer
	var labels Alabels
	if opts.WithoutTags == false {
		if jsons, err := os.ReadFile(opts.DelugeDir + "label.conf"); err != nil {
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
		if err := json.Unmarshal(jsn.Bytes(), &labels); err != nil {
			opts.WithoutTags = true
		}
	}

	// prepare replaces
	var replaces []*Replace
	for _, str := range opts.Replaces {
		patterns := strings.Split(str, ",")
		replaces = append(replaces, &Replace{
			From: patterns[0],
			To:   patterns[1],
		})
	}

	for fastResumeHashId, value := range fastResumeFile {
		var fastResume qBittorrentStructures.QBittorrentFastresume
		if err := bencode.DecodeString(value.(string), &fastResume); err != nil {
			torrentFile := torrentStructures.Torrent{}
			torrentFilePath := opts.DelugeDir + "state" + sep + fastResumeHashId + ".torrent"
			if _, err = os.Stat(torrentFilePath); os.IsNotExist(err) {
				chans.errChannel <- fmt.Sprintf("Can't find torrent file %v. Can't decode string %v. Continue", torrentFilePath, fastResumeHashId)
				continue
			}
			err = helpers.DecodeTorrentFile(torrentsPath, torrentFile)
			if err != nil {
				chans.errChannel <- fmt.Sprintf("Can't decode torrent file %v. Can't decode string %v. Continue", torrentFilePath, fastResumeHashId)
				continue
			}
			log.Printf("Can't decode row %v with torrent %v. Continue", fastResumeHashId, torrentFile.Info.Name)
		}
		wg.Add(1)
		chans.boundedChannel <- true
		go logic(fastResumeHashId, fastResume, &opts, &chans, &torrentsPath, &wg, &labels, replaces)
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

	var errorsExist bool
	for message := range chans.errChannel {
		log.Printf("%v \n", message)
		errorsExist = true
	}

	fmt.Println()
	log.Println("Ended")
	if errorsExist {
		log.Println("Not all torrents was processed")
	}
	fmt.Println("\nPress Enter to exit")
	fmt.Scanln()
}
