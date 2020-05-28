package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"

	pb "github.com/elcuervo/tangalanga/proto"
	"github.com/golang/protobuf/proto"

	"github.com/briandowns/spinner"
	"github.com/cretz/bine/tor"
	"github.com/ipsn/go-libtor"
)

const logo = `
▄▄▄▄▄ ▄▄▄·  ▐ ▄  ▄▄ •  ▄▄▄· ▄▄▌   ▄▄▄·  ▐ ▄  ▄▄ •  ▄▄▄·
•██  ▐█ ▀█ •█▌▐█▐█ ▀ ▪▐█ ▀█ ██•  ▐█ ▀█ •█▌▐█▐█ ▀ ▪▐█ ▀█
 ▐█.▪▄█▀▀█ ▐█▐▐▌▄█ ▀█▄▄█▀▀█ ██▪  ▄█▀▀█ ▐█▐▐▌▄█ ▀█▄▄█▀▀█
 ▐█▌·▐█ ▪▐▌██▐█▌▐█▄▪▐█▐█ ▪▐▌▐█▌▐▌▐█ ▪▐▌██▐█▌▐█▄▪▐█▐█ ▪▐▌
 ▀▀▀  ▀  ▀ ▀▀ █▪·▀▀▀▀  ▀  ▀ .▀▀▀  ▀  ▀ ▀▀ █▪·▀▀▀▀  ▀  ▀
`

const zoomUrl = "https://www3.zoom.us/conf/j"

var colorFlag = flag.Bool("colors", true, "enable or disable colors")
var token = flag.String("token", "", "zpk token to use")
var color aurora.Aurora

func init() {
	rand.Seed(time.Now().UnixNano())
	color = aurora.NewAurora(*colorFlag)
	flag.Parse()

	if *token == "" {
		log.Panic("Missing token")
	}

	log.Println(color.Green(logo))
}

func debugReq(req *http.Request) {
	requestDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		fmt.Println(err)
	}

	log.Println(string(requestDump))
}

func randomMeetingId() int {
	//     88392789130
	min := 80000000000
	max := 99999999999

	return rand.Intn(max-min+1) + min
}

type Tangalanga struct {
	client *http.Client
}

func (t *Tangalanga) FindMeeting(id int) (*pb.Meeting, error) {
	p := url.Values{"cv": {"5.0.25694.0524"}, "mn": {strconv.Itoa(id)}, "uname": {"tangalanga"}}

	req, _ := http.NewRequest("POST", zoomUrl, strings.NewReader(p.Encode()))
	cookie := fmt.Sprintf("zpk=%s", *token)

	req.Header.Add("Cookie", cookie)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, _ := t.client.Do(req)
	body, _ := ioutil.ReadAll(resp.Body)

	m := &pb.Meeting{}
	err := proto.Unmarshal(body, m)
	if err != nil {
		log.Panic("err: ", err)
	}

	missing := m.GetError() != 0

	if missing {
		info := m.GetInformation()

		if m.GetError() == 124 {
			log.Panic(info)
		}

		return nil, fmt.Errorf("Error: %s", color.Red(info))
	}

	return m, nil
}

func main() {
	client := &http.Client{}

	tangalanga := &Tangalanga{
		client: client,
	}

	for i := 0; i < 200; i++ {
		id := randomMeetingId()

		m, err := tangalanga.FindMeeting(id)

		if err != nil {
			log.Println(err)
		} else {
			room := m.GetRoom()
			log.Printf("Found ID: %d Hello: %s", color.Green(room.GetRoomId()), color.Green(room.GetRoomName()))
		}
	}
}

func main2() {
	s := spinner.New(spinner.CharSets[4], 100*time.Millisecond)
	c := make(chan os.Signal, 2)

	s.Suffix = " Connecting to the TOR network."
	s.Start()

	go func() {
		<-c
		os.Exit(0)
	}()

	t, err := tor.Start(nil, &tor.StartConf{ProcessCreator: libtor.Creator})

	if err != nil {
		fmt.Errorf("Unable to start Tor: %v", err)
	}

	defer t.Close()

	dialCtx, dialCancel := context.WithTimeout(context.Background(), 3*time.Minute)

	dialer, err := t.Dialer(dialCtx, nil)

	httpClient := &http.Client{Transport: &http.Transport{DialContext: dialer.DialContext}}

	tangalanga := &Tangalanga{
		client: httpClient,
	}

	tangalanga.FindMeeting(1)

	defer dialCancel()

	s.Stop()
}
