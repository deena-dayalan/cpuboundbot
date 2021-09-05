package main

import (
	"log"
	"math"
	"net/smtp"
	"time"
	"github.com/atc0005/go-teams-notify/v2"
	"bytes"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
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

type mailDetails struct {
	UserName string
	HostName string
	Message  string
	Usage    string
}

//Global variables
var hostName string
var currentFile string
var LOG_FILE string = "/work/rc/cpuboundbot/cpubot_"
var rc_email = "rchelp@northeastern.edu"
var templateFile = "emailTemplate.html"

const SLICES = "/sys/fs/cgroup/cpu/user.slice"

// url list
const webhookUrl = "https://northeastern.webhook.office.com/webhookb2/8ec2dc62-2e7c-4cf2-882b-e8fe8e4f3c3f@a8eec281-aaa3-4dae-ac9b-9a398b9215e7/IncomingWebhook/90f15f2d7a9d4383b10e0415f8e975bb/ed3c5fea-f873-40fa-8474-573432ab0a58"

//Log errors
func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

//Removes duplicate entries and returns unique list
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

//Returns cpuacct usage information
func CgLs(s string) string {
	p := SLICES + "/user-" + s + ".slice"
	out, err := exec.Command("systemd-cgls", p).Output()
	check(err)
	return string(out)
}

//Returns UNIX time
func timeCheck() int64 {
	now := time.Now()
	timeStamp := now.Unix()
	return timeStamp
}

//Check if string exist in the file and returns True or False
func IsExist(str, filepath string) bool {
	b, err := ioutil.ReadFile(filepath)
	check(err)
	isExist, err := regexp.Match(str, b)
	check(err)
	return isExist
}

//Send alert email to the violators
func alertEmail(details mailDetails) (string, string) {
	to := []string{details.UserName + "@northeastern.edu"}
	to = append(to, rc_email)
	subject := "Subject: Discovery - Login Node Use Warning (" + details.UserName + ")\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	template, _ := template.ParseFiles(templateFile)
	var body bytes.Buffer
	template.Execute(&body, details)
	msg := []byte(subject + mime + body.String())
	err := smtp.SendMail("smtp.discovery.neu.edu:25", nil, rc_email, to, msg)
	notify := "notified"
	timeNow := timeCheck()
	if err != nil {
		notify = "un-notified"
		check(err)
	}
	timeString := strconv.FormatInt(timeNow, 10)
	return notify, timeString
}

//Returns notified and unnotified users
func notification(filepath string) ([]string, []string) {
	content, err := ioutil.ReadFile(filepath)
	check(err)
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
				notifd = append(notifd, words[1]+","+words[2]+","+words[4])
			}
		}
	}
	return notifd, unNotifd
}

//Converts and returns UNIX time to YYYY-MM-DD HH:MM:SS
func timeConversion(unixTime string) string {
	uTime, _ := strconv.ParseInt(unixTime, 10, 64)
	dateTime := time.Unix(uTime, 0)
	loc, _ := time.LoadLocation("America/New_York")
	newTime := dateTime.In(loc).Format("2006-01-02 15:04:05")
	return newTime
}

//Triggers alert message to RC teams channel
func teamsMessage(filepath string) {
	notifd, unNotifd := notification(filepath)
	mstClient := goteamsnotify.NewClient()
	msgCard := goteamsnotify.NewMessageCard()
	//msgCard.Title = "CPU high usage alert!!!"
	msgCard.Title = "Discovery - High CPU Usage Alert!!!"
	messageText := "Notice: User(s) performing cpu intensive task on " + hostName + " node.<br>"
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
			notified := timeConversion(word[2])
			userList += "Username: " + word[0] + " Usage: " + word[1] + " Notified: " + notified + "<br>"
		}
		messageText = messageText + "Email sent to the following user(s).<br>" + userList + "<br>"
	}
	msgCard.Text = messageText
	msgCard.ThemeColor = "#DF813D"
	mstClient.Send(webhookUrl, msgCard)
}

//Writes string in a file
func writeLog(line string, file string) {
	logLine := line
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	check(err)
	defer f.Close()
	_, err = f.WriteString(logLine + "\n")
	check(err)
}

func compareUsage(old Reading, new Reading, maxusage int) {
	duration := new.Timestamp.Sub(old.Timestamp)
	for uid, cpu := range new.Entries {
		used := cpu - old.Entries[uid]
		usage := int(math.Round(float64(used) / float64(duration) * 100))
		if usage >= maxusage {
			user, err := user.LookupId(uid)
			check(err)
			username := user.Username
			cpuUsage := strconv.Itoa(usage) + "%"
			infractions := CgLs(uid)
			//writing current log to validate against the main log
			violator := uid + "," + username + "," + cpuUsage
			writeLog(violator, currentFile)
			checkId := IsExist(uid, LOG_FILE)
			if !checkId {
				//UID is new entry
				details := mailDetails{username, hostName, infractions, cpuUsage}
				notice, timeStamp := alertEmail(details)
				line := uid + "," + username + "," + cpuUsage + "," + notice + "," + timeStamp
				writeLog(line, LOG_FILE)
			} else {
				//UID already exist in file; then check for "notified" flag
				input, err := ioutil.ReadFile(LOG_FILE)
				check(err)
				lines := strings.Split(string(input), "\n")
				for i, line := range lines {
					if strings.Contains(line, uid) {
						word := strings.Split(lines[i], ",")
						notice := word[3]
						sent, _ := strconv.ParseInt(word[4], 10, 64)
						timeNow := timeCheck()
						timeCheck := timeNow - sent
						notifyCheck := strings.Compare(notice, "un-notified")
						if notifyCheck == 0 || timeCheck >= 3600 {
							details := mailDetails{username, hostName, infractions, cpuUsage}
							notice, timeStamp := alertEmail(details)
							line := uid + "," + username + "," + cpuUsage + "," + notice + "," + timeStamp
							lines[i] = line
						}
					}
				}
				output := strings.Join(lines, "\n")
				err = ioutil.WriteFile(LOG_FILE, []byte(output), 0644)
				check(err)
			}
		}
	}

}

//Returns user list with associated cpuacct usage
func New() Reading {
	now := time.Now()
	m := make(map[string]int)
	re := regexp.MustCompile(`^user-(\d+)\.slice`)
	files, err := ioutil.ReadDir(SLICES)
	check(err)
	for _, f := range files {
		n := f.Name()
		if re.Match([]byte(n)) {
			uid := re.FindStringSubmatch(n)[1]
			usage := readEntry(SLICES + "/" + n + "/cpuacct.usage")
			m[uid] = usage
		}
	}
	return Reading{hostName, now, m}
}

//Reads and returns cpuacct usage information from cpuacct.usage file
func readEntry(s string) int {
	contents, err := ioutil.ReadFile(s)
	check(err)
	data := chomp(string(contents))
	x, err := strconv.Atoi(data)
	check(err)
	return x
}

//Returns substring
func chomp(s string) string {
	x := len(s)
	return s[:x-1]
}

//Compare main log and current log to keep track of violators
func logCompare(mainLog string, currLog string) {
	//if current log is empty (no violators); then cat /dev/null > mainlog
	if _, err := os.Stat(currLog); err != nil {
		if err := os.Truncate(mainLog, 0); err != nil {
			log.Printf("Failed to truncate main log: %v", err)
		}
	} else {
		input, err := ioutil.ReadFile(mainLog)
		check(err)
		lines := strings.Split(string(input), "\n")
		var newLines []string
		for i, line := range lines {
			word := strings.Split(lines[i], ",")
			UID := word[0]
			checkId := IsExist(UID, currLog)
			if checkId {
				newLines = append(newLines, line)
			}
		}
		output := strings.Join(newLines, "\n")
		err = ioutil.WriteFile(mainLog, []byte(output), 0644)
		check(err)
	}

}

func main() {
	hostName, _ = os.Hostname()
	LOG_FILE = LOG_FILE + hostName
	currentFile = LOG_FILE + "_current"

	//creating main log file
	_, err := os.Create(LOG_FILE)
	check(err)

	//Deleting current log if already present
	if _, err := os.Stat(currentFile); err == nil {
		os.Remove(currentFile)
	}

	for {

		c := Config{Period: "5m", MaxUsage: 35}
		period, err := time.ParseDuration(c.Period)
		check(err)

		// CPU usage reading 1
		baseReading := New()

		//sleep
		time.Sleep(period)

		// CPU usage reading 2
		newReading := New()

		// compare reading 1 & 2 and check cpuacct usage % with threshold
		compareUsage(baseReading, newReading, c.MaxUsage)

		//delete entires in LOG_FILE if not present in current log; means the violater don't exist anymore
		if fileCheck, err := os.Stat(LOG_FILE); err == nil {
			if fileCheck.Size() > 0 {
				logCompare(LOG_FILE, currentFile)
			}
		}

		//LOG_FILE is not empty; then send alert to RC Teams channel
		if fileCheck, err := os.Stat(LOG_FILE); err == nil {
			if fileCheck.Size() > 0 {
				teamsMessage(LOG_FILE)
			}
		}

		//Deleting current log
		if _, err := os.Stat(currentFile); err == nil {
			os.Remove(currentFile)
		}

		//sleep
		time.Sleep(period)
	}
}
