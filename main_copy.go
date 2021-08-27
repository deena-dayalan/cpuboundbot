package main

import (
	"log"
	"math"
	"net/smtp"
	"time"

	//"github.com/atc0005/go-teams-notify/v2"
	"bufio"
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

func logCompare(logNew string, logOld string) string {

	oldFile, err := os.OpenFile(logOld, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Println(err)
	}
	defer oldFile.Close()
	readOld := bufio.NewReader(oldFile)
	lines := ""
	for {
		lineOld, err := readOld.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("log file read line error: %v", err)
		}
		if len(lineOld) > 0 {
			word := strings.Split(lineOld, ",")
			uid := word[0]
			check := IsExist(uid, logNew)
			if check {
				lines += lineOld + "/n"
			}
		}
	}

	if len(lines) > 0 {
		if err := os.Truncate(logOld, 0); err != nil {
			log.Printf("Failed to truncate: %v", err)
		}
		writeLog(lines, logOld)
	}
	return ""
}

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

func alertEmail(username string, message string, usage string) (string, string) {

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
	timeString := strconv.Itoa(timeNow)
	return notify, timeString
}

func notification(filepath string) ([]string, []string) {
	content, err := ioutil.ReadFile(filepath)
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

func teamsMessage(filepath string) {
	notifd, unNotifd := notification(filepath)
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

func alerting(orgLog string, prevLog string) {

	file, err := os.OpenFile(orgLog, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Println(err)
	}
	defer file.Close()
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
			uid := word[0]
			username := word[1]
			usage := word[2]
			infractions := CgLs(uid)
			if _, error := os.Stat(prevLog); error == nil {
				//prev file is not empty
				check := IsExist(uid, prevLog)
				if !check {
					notice, timeStamp := alertEmail(username, infractions, usage)
					newLine := uid + "," + username + "," + usage + "," + notice + "," + timeStamp
					writeLog(newLine, prevLog)
				}

			} else {
				//prev file empty; write prev file
				notice, timeStamp := alertEmail(username, infractions, usage)
				newLine := uid + "," + username + "," + usage + "," + notice + "," + timeStamp
				writeLog(newLine, prevLog)
			}
		}
	}

}

func writeLog(line string, file string) {

	logLine := line
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
			cpuUsage := strconv.Itoa(usage) + "%"
			line := uid + "," + username + "," + cpuUsage
			writeLog(line, LOG_FILE)
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
	prevFile := LOG_FILE + "_bkp"

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

			alerting(LOG_FILE, prevFile)

			logCompare(LOG_FILE, prevFile)

			teamsMessage(prevFile)

			if err := os.Truncate(LOG_FILE, 0); err != nil {
				log.Printf("Failed to truncate: %v", err)
			}
		}

		time.Sleep(period)
	}
}
