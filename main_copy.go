package main

import (
	"log"
	"math"
	"net/smtp"
	"time"

	"github.com/atc0005/go-teams-notify/v2"

	"fmt"
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

//Global variables
var hostName string
var currentFile string
var LOG_FILE string = "/home/d.dasarathan/cpuboundbot/cpubot_test_"
var rc_email_list = "dasarathan.d@northeastern.edu"

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

	from := "rchelp@northeastern.edu"
	to := []string{username + "@northeastern.edu"}
	to = append(to, rc_email_list)
	//subject := "Subject: CPUBoundBot notice (" + username + ")\n"
	subject := "Subject: Discovery - CPUBoundBot notice \n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	body := "<html><body>Dear " + username + ",<br><br>We have noticed that you're running an intensive CPU-bound activity on one of the login nodes(" + hostName + "), as detailed below." + "<br>" +
		"<pre>" + message + "</pre><br>" + "CPU usage: " + usage + "<br>" +
		"The login node should not be used for CPU intensive activities, as this can impact the performance of this node. " + "<br>" +
		"It also will not give you the best performance for the tasks you are trying to do. " +
		"Please check our <a href=" + "https://rc-docs.northeastern.edu/en/latest/get_started/connect.html#next-steps" + ">Next setps</a> documentation." + "<br><br>" +
		"If you are trying to run a job, you should move to a compute node. " +
		"You can do this interactively using the srun command or non-interactively using sbatch command. " +
		"Please see our documentation on how to do this:  " +
		"<a href=" + "https://rc-docs.northeastern.edu/en/latest/using-discovery/sbatch.html" + ">Using sbatch</a>; " +
		"<a href=" + "https://rc-docs.northeastern.edu/en/latest/using-discovery/srun.html" + ">Using srun</a>.<br><br>" +
		"If you are trying to transfer data, we have a dedicated transfer node that you should use. Please see our documentation on transferring data for more information: " +
		"<a href=" + "https://rc-docs.northeastern.edu/en/latest/using-discovery/transferringdata.html" + ">Transferring Data</a>." + "<br><br>" +
		"If you have any questions or need further assistance, feel free to email or book a consultation with us." + "<br>"

	tail := "<br>" + "Thanks," + "<br>" + "The Research Computing Team" + "<br>" + "Northeastern University." + "<br>" +
		"<a href=" + "mailto:rchelp@northeastern.edu" + ">rchelp@northeastern.edu</a>" + "<br>" +
		"<a href=" + "https://outlook.office365.com/owa/calendar/ResearchComputing2@northeastern.onmicrosoft.com/bookings/" + ">Book your appointment</a>" + "</body></html>"
	msg := []byte(subject + mime + body + tail)

	err := smtp.SendMail("smtp.discovery.neu.edu:25", nil, from, to, msg)
	notify := "notified"
	timeNow := timeCheck()
	if err != nil {
		log.Println(err)
		notify = "un-notified"
	}
	timeString := strconv.FormatInt(timeNow, 10)
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
				notifd = append(notifd, words[1]+","+words[2]+","+words[4])
			}
		}
	}
	return notifd, unNotifd
}

func timeConversion(unixTime string) string {
	uTime, _ := strconv.ParseInt(unixTime, 10, 64)
	dateTime := time.Unix(uTime, 0)
	loc, _ := time.LoadLocation("America/New_York")
	newTime := dateTime.In(loc).Format("2006-01-02 15:04:05")
	return newTime
}

func teamsMessage(filepath string) {
	notifd, unNotifd := notification(filepath)
	mstClient := goteamsnotify.NewClient()
	msgCard := goteamsnotify.NewMessageCard()
	//msgCard.Title = "CPU high usage alert" for testing by deena
	msgCard.Title = "Test alert please ignore"
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
			//userList += "Username: " + word[0] + " Usage: " + word[1] + "<br>"
			userList += "Username: " + word[0] + " Usage: " + word[1] + " Notified: " + notified + "<br>"
		}
		messageText = messageText + "Email sent to the following user(s).<br>" + userList + "<br>"
	}
	msgCard.Text = messageText
	msgCard.ThemeColor = "#DF813D"
	mstClient.Send(webhookUrl, msgCard)
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
	fmt.Println("compareUsage")
	duration := new.Timestamp.Sub(old.Timestamp)
	for uid, cpu := range new.Entries {
		used := cpu - old.Entries[uid]
		usage := int(math.Round(float64(used) / float64(duration) * 100))

		if usage >= maxusage {
			user, err := user.LookupId(uid)
			if err != nil {
				log.Fatalf("Unable to resolve uid %s\n", uid)
			}
			username := user.Username
			cpuUsage := strconv.Itoa(usage) + "%"
			infractions := CgLs(uid)
			//writing current log to validate against the main log
			violator := uid + "," + username + "," + cpuUsage
			fmt.Println("compareUsage usage >= max usage", violator)
			writeLog(violator, currentFile)
			check := IsExist(uid, LOG_FILE)
			if !check {
				//UID is new entry
				fmt.Println("compareUsage---check false..UID do not exist in main log")
				notice, timeStamp := alertEmail(username, infractions, cpuUsage)
				line := uid + "," + username + "," + cpuUsage + "," + notice + "," + timeStamp
				writeLog(line, LOG_FILE)
			} else {
				//UID already exist in file; then check for "notified" flag
				fmt.Println("compareUsage---check true..UID already exist in main log")
				input, err := ioutil.ReadFile(LOG_FILE)
				if err != nil {
					log.Fatalln(err)
				}
				lines := strings.Split(string(input), "\n")
				for i, line := range lines {
					if strings.Contains(line, uid) {
						word := strings.Split(lines[i], ",")
						notice := word[3]
						sent, _ := strconv.ParseInt(word[4], 10, 64)
						timeNow := timeCheck()
						timeCheck := timeNow - sent
						notifyCheck := strings.Compare(notice, "un-notified")
						//if notifyCheck == 0 || timeCheck >= 600 {  --commented by deena
						if notifyCheck == 0 || timeCheck >= 60 {
							fmt.Println("compareUsage---check true..UID already exist in main log...un-notified or time check greater than 600")
							notice, timeStamp := alertEmail(username, infractions, cpuUsage)
							line := uid + "," + username + "," + cpuUsage + "," + notice + "," + timeStamp
							lines[i] = line
						}
					}
				}
				output := strings.Join(lines, "\n")
				err = ioutil.WriteFile(LOG_FILE, []byte(output), 0644)
				if err != nil {
					log.Fatalln(err)
				}
			}
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
	return Reading{hostName, now, m}
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

func logCompare(mainLog string, currLog string) {
	//if current lof is empty (no violators); then cat /dev/null > mainlog
	if _, err := os.Stat(currLog); err != nil {
		fmt.Println("logCompare---current log is empty")
		if err := os.Truncate(mainLog, 0); err != nil {
			fmt.Println("logCompare---current log is empty..truncate main log")
			log.Printf("Failed to truncate main log: %v", err)
		}
	} else {
		fmt.Println("logCompare---current log is not empty")
		input, err := ioutil.ReadFile(mainLog)
		if err != nil {
			log.Fatalln(err)
		}
		lines := strings.Split(string(input), "\n")
		var newLines []string
		for i, line := range lines {
			word := strings.Split(lines[i], ",")
			UID := word[0]
			check := IsExist(UID, currLog)
			if check {
				fmt.Println("logCompare---current log is not empty...line exit in current log")
				fmt.Println(line)
				newLines = append(newLines, line)
			}
		}
		output := strings.Join(newLines, "\n")
		err = ioutil.WriteFile(mainLog, []byte(output), 0644)
		if err != nil {
			log.Fatalln(err)
		}
	}

}

func main() {

	hostName, _ = os.Hostname()
	LOG_FILE = LOG_FILE + hostName
	currentFile = LOG_FILE + "_current"

	//creating main log file
	_, e := os.Create(LOG_FILE)
	if e != nil {
		log.Fatal(e)
	}

	//Deleting current log if already present
	if _, err := os.Stat(currentFile); err == nil {
		os.Remove(currentFile)
	}

	for {

		c := Config{Period: "5m", MaxUsage: 35} 
		period, err := time.ParseDuration(c.Period)
		if err != nil {
			log.Fatalf("unable to parse period")
		}
		// CPU usage reading 1
		baseReading := New()
		fmt.Println("BaseReading: ", baseReading)
		//sleep
		time.Sleep(period)
		// CPU usage reading 2
		newReading := New()
		fmt.Println("NewReading: ", newReading)

		// compare reading 1 & 2 and check with threshold
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
