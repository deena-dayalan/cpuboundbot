package main

import (
	"log"
	"math"
	"net/smtp"
	"time"

	//"github.com/atc0005/go-teams-notify/v2"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"time"

	//"os/exec"
	"fmt"
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
var rc_email_list = "d.dasarathan@northeastern.edu"

//const mailedUsers = "/home/d.dasarathan/cpuboundbot/mailedUsers"
const SLICES = "/sys/fs/cgroup/cpu/user.slice"
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

func timeCheck() int64 {
	now := time.Now()
	timeStamp := now.Unix()
	return timeStamp
}

/*func mailLog(username string, timenow int64) {
	tmt := strconv.Itoa(timenow)
	logLine := username + "," + tmt
	f, err := os.OpenFile(mailedUsers, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString(logLine + "\n"); err != nil {
		log.Println(err)
	}

}*/

func logCopy(originalFile string) string {

	original, err := os.Open(originalFile)
	if err != nil {
		log.Fatal(err)
	}
	defer original.Close()

	// Create new file
	newFile := LOG_FILE + "_bkp"
	new, err := os.Create(newFile)
	if err != nil {
		log.Fatal(err)
	}
	defer new.Close()

	//This will copy
	if _, err := io.Copy(new, original); err != nil {
		log.Fatal(err)
	}
	return newFile
}

/*func logRecycle(filepath string) {

	newFile := logCopy(filepath)
	file, err := os.OpenFile(newFile, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Println(err)
	}
	timenow := timeCheck()
	read := bufio.NewReader(file)
	for {
		line, err := read.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("log file read line error: %v", err)
		}
		if len(line) > 0 {
			word := strings.Split(line, ",")
			fmt.Println("5: ", word[5])
			prev, _ := strconv.ParseInt(word[5], 10, 64)
			if timenow-prev < 601 {
				uid := word[0]
				username := word[1]
				usage := word[2]
				notify := word[3]
				//host := word[4]
				tmt := word[5]
				writeLog(uid, username, usage, notify, tmt)
			}

		}
	}

}*/

func IsExist(str, filepath string) bool {
	b, err := ioutil.ReadFile(filepath)
	if err != nil {
		panic(err)
	}
	isExist, err := regexp.Match(str, b)
	if err != nil {
		panic(err)
	}
	return isExist
}

func alertEmail(username string, message string, usage string) (string, int64) {

	to := []string{username + "@northeastern.edu"}
	to = append(to, rc_email_list)
	msg := []byte("To: Discovery Alerts - Research Computing\r\n" +
		"Subject: cpuboundbot notice (" + username + ")\r\n" +
		"\r\n" + message + "\r\n" + "Usage: " + usage + "\r\n" +
		"Please do not run cpu intensive tasks on login nodes." + "\n\n" + "Thanks," + "\n" + "Research Computing.")
	err := smtp.SendMail("smtp.discovery.neu.edu:25", nil, "CPUBoundBot@discovery.neu.edu", to, msg)
	notify := "notified"
	timeNow := timeCheck()
	if err != nil {
		log.Println(err)
		notify = "un-notified"
	}
	return notify, timeNow
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

/*func writeLog(uid string, username string, usage string, notify string, timeStamp string) {

	//tmt := strconv.Itoa(timeStamp)
	logLine := uid + "," + username + "," + usage + "," + notify + "," + host + "," + timeStamp
	f, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString(logLine + "\n"); err != nil {
		log.Println(err)
	}

}*/

func writeLog(line string) {

	logLine := line
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

	for uid, cpu := range new.Entries {
		used := cpu - old.Entries[uid]
		usage := int(math.Round(float64(used) / float64(duration) * 100))
		if usage > maxusage {
			user, err := user.LookupId(uid)
			if err != nil {
				log.Fatalf("Unable to resolve uid %s\n", uid)
			}
			username := user.Username
			//infractions := CgLs(uid)
			//check := IsExist(username, LOG_FILE)
			//notice := "notified"
			//timeStamp := timeCheck()
			cpuUsage := strconv.Itoa(usage) + "%"
			//tmt := strconv.Itoa(timeStamp)
			/*if !check {
				notice, timeStamp = alertEmail(username, infractions, cpuUsage)
			}*/
			//writeLog(uid, username, usage, notice, timeStamp)
			line := uid + "," + username + "," + cpuUsage
			writeLog(line)
		}
	}

}

func New() Reading {

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
	return Reading{host, now, m}
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

	host, err := os.Hostname()
	if err != nil {
		log.Fatalf("Unable to determine hostname\n")
	}
	LOG_FILE = LOG_FILE + host

	for {

		c := Config{Period: "5m", MaxUsage: 35}
		period, err := time.ParseDuration(c.Period)
		if err != nil {
			log.Fatalf("unable to parse period")
		}

		baseReading := New()
		fmt.Println("BaseReading: ", baseReading)
		time.Sleep(period)
		newReading := New()
		fmt.Println("NewReading: ", newReading)

		compareUsage(baseReading, newReading, c.MaxUsage)

		if _, err := os.Stat(LOG_FILE); err == nil {

			bkpLog := logCopy(LOG_FILE)

			teamsMessage()
		}

		//logRecycle(LOG_FILE)

		time.Sleep(period)
	}
}
