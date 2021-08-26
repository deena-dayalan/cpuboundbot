package main

import (
	"io/ioutil"
	"log"
	"math"
	"net/smtp"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	//"os/exec"
	"regexp"
	"strconv"
)

type Config struct {
	Period   string
	MaxUsage int
}

type Reading struct {
	Hostname  string
	Timestamp time.Time
	Entries   map[string]int
}

//Global variables
var host string
var LOG_FILE string = "/home/d.dasarathan/cpuboundbot/cpubot"

var rc_email_list = []string{"d.dasarathan@northeastern.edu"}

const SLICES = "/sys/fs/cgroup/cpu/user.slice"

//const LOG_FILE = "/home/d.dasarathan/cpuboundbot/cpubot"
const webhookUrl = "https://northeastern.webhook.office.com/webhookb2/8ec2dc62-2e7c-4cf2-882b-e8fe8e4f3c3f@a8eec281-aaa3-4dae-ac9b-9a398b9215e7/IncomingWebhook/90f15f2d7a9d4383b10e0415f8e975bb/ed3c5fea-f873-40fa-8474-573432ab0a58"

func unique(stringSlice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range stringSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func CgLs(s string) string {
	p := SLICES + "/user-" + s + ".slice"
	out, err := exec.Command("systemd-cgls", p).Output()
	if err != nil {
		log.Fatal(err)
	}
	return string(out)
}

func alertEmail(username string, message string, usage int) string {
	to := []string{username + "@northeastern.edu"}
	to = append(to, rc_email_list)
	msg := []byte("To: Discovery Alerts - Research Computing\r\n" +
		"Subject: cpuboundbot notice (" + username + ")\r\n" +
		"\r\n" + message + "\r\n" + "Usage: " + string(usage) + "%\r\n" +
		"Please do not run cpu intensive tasks on login nodes")
	err := smtp.SendMail("smtp.discovery.neu.edu:25", nil, "CPUBoundBot@discovery.neu.edu", to, msg)
	notify := "notified"
	if err != nil {
		log.Println(err)
		notify = "un-notified"
	}
	return notify
}

func notification() ([]string, []string) {
	content, err := ioutil.ReadFile(LOG_FILE)
	if err != nil {
		log.Println(err)
	}
	lines := strings.Split(string(content), "\n")
	unqLines := unique(lines)
	notifd := []string{}
	unNotifd := []string{}
	for _, line := range unqLines {
		if line != "" {
			words := strings.Split(string(line), ",")
			if words[3] == "un-notified" {
				unNotifd = append(unNotifd, words[1]+","+words[2])
			} else if words[3] == "notified" {
				notifd = append(notifd, words[1]+","+words[2])
			}
		}
	}
	return notifd, unNotifd
}

func teamsMessage() {
	notifd, unNotifd := notification()
	mstClient := goteamsnotify.NewClient()
	msgCard := goteamsnotify.NewMessageCard()
	msgCard.Title = "CPU high usage alert"
	messageText := "Notice: User(s) performing cpu intensive task on " + host + " node.<br>"
	if len(unNotifd) > 0 {
		userList := ""
		for i := 0; i < len(unNotifd); i++ {
			word := strings.Split(unNotifd[i], ",")
			userList += "Username: " + word[0] + " Usage: " + word[1] + "<br>"
		}
		messageText = messageText + "Email not sent to the following user(s).<br>" + userList + "<br>"
	} else if len(notifd) > 0 {
		userList := ""
		for i := 0; i < len(notifd); i++ {
			word := strings.Split(notifd[i], ",")
			userList += "Username: " + word[0] + " Usage: " + word[1] + "<br>"
		}
		messageText = messageText + "Email sent to the following user(s).<br>" + userList + "<br>"
	}
	msgCard.Text = messageText
	msgCard.ThemeColor = "#DF813D"
	mstClient.Send(webhookUrl, msgCard)
}

func writeLog(uid string, username string, used int, notify string) {
	usage := strconv.Itoa(used)
	LOG_FILE = LOG_FILE + host
	logLine := uid + "," + username + "," + usage + "%" + "," + notify + "," + host
	f, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString(logLine + "\n"); err != nil {
		log.Println(err)
	}

}

func compareUsage(old Reading, new Reading, maxusage int) {

	duration := new.Timestamp.Sub(old.Timestamp)
	host = new.Hostname
	for uid, cpu := range new.Entries {
		if uid == "1825610918" {
			used := cpu - old.Entries[uid]
			usage := int(math.Round(float64(used) / float64(duration) * 100))
			if usage > maxusage {
				//fmt.Printf("user %s: %d percent\n", uid, usage)
				user, err := user.LookupId(uid)
				if err != nil {
					log.Fatalf("Unable to resolve uid %s\n", uid)
				}
				username := user.Username
				infractions := CgLs(uid)
				notice := alertEmail(username, infractions, usage)
				writeLog(uid, username, usage, notice)
			}
		}
	}

}

func New() Reading {

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to determine hostname\n")
	}

	now := time.Now()
	m := make(map[string]int)
	re := regexp.MustCompile(`^user-(\d+)\.slice`)

	files, err := ioutil.ReadDir(SLICES)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {

		n := f.Name()
		if re.Match([]byte(n)) {
			uid := re.FindStringSubmatch(n)[1]
			usage := readEntry(SLICES + "/" + n + "/cpuacct.usage")
			m[uid] = usage
		}
	}
	return Reading{hostname, now, m}
}

func readEntry(s string) int {

	contents, err := ioutil.ReadFile(s)
	if err != nil {
		log.Fatalf("error reading %s\n", s)
		return 0
	}
	data := chomp(string(contents))
	x, err := strconv.Atoi(data)
	if err != nil {
		return 0
	}
	return x
}

func chomp(s string) string {
	x := len(s)
	return s[:x-1]
}

func main() {

	for {

		if _, err := os.Stat(LOG_FILE); err == nil {
			os.Remove(LOG_FILE)
		}

		c := Config{Period: "5m", MaxUsage: 35}
		period, err := time.ParseDuration(c.Period)
		if err != nil {
			log.Fatalf("unable to parse period")
		}

		baseReading := New()
		time.Sleep(period)
		newReading := New()
		compareUsage(baseReading, newReading, c.MaxUsage)

		if _, err := os.Stat(LOG_FILE); err == nil {
			teamsMessage()
			os.Remove(LOG_FILE)
		}

		time.Sleep(period)
	}
}
