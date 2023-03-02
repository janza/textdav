package crapdav

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/tj/go-naturaldate"
)

var headersRegex *regexp.Regexp

func init() {
	headersRegex = regexp.MustCompile(`#+ *(✔️)? *(?P<heading>[^{]*) *{?#?(?P<id>[^}]*)`)
}

func IcalToMarkdown(tmpFile io.Writer, events []*ical.Component) error {
	for idx, event := range events {
		summary := Get(event, ical.PropSummary)
		desc := GetText(event, ical.PropDescription)
		due, err := event.Props.DateTime("due", time.Local)
		dueTime := ""
		if err == nil && !due.IsZero() {
			dueTime = "@" + due.Local().Format("2006-01-02 15:04") + "\n"
		}

		status := event.Props.Get(ical.PropStatus)

		if status != nil && status.Value == "COMPLETED" {
			summary = "✔️ " + summary
			continue
		}

		if desc != "" {
			desc += "\n"
		}

		fmt.Fprintf(tmpFile, "# %s {#%d}\n%s%s\n", summary, idx, dueTime, desc)
	}
	return nil
}

func createEvent() *ical.Component {
	uid := uuid.New().String()
	component := ical.NewComponent(ical.CompToDo)
	component.Props.SetText(ical.PropUID, uid)
	component.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())
	component.Props.SetDateTime(ical.PropCreated, time.Now().UTC())

	return component
}

func encodeEvent(component *ical.Component, eventblock *EventBlock) []byte {
	component.Props.SetText(ical.PropStatus, strings.TrimSpace(eventblock.Status))
	component.Props.SetText(ical.PropSummary, strings.TrimSpace(eventblock.Summary))
	component.Props.SetText(ical.PropDescription, strings.TrimSpace(eventblock.Description))
	if !eventblock.Due.IsZero() {
		component.Props.SetDateTime(ical.PropDue, *eventblock.Due)
	}
	component.Props.SetDateTime(ical.PropLastModified, time.Now())

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "dev.jjanzic.crapdav")
	cal.Children = append(cal.Children, component)

	var buf bytes.Buffer
	encoder := ical.NewEncoder(&buf)
	if err := encoder.Encode(cal); err != nil {
		log.Fatal(err)
	}

	return buf.Bytes()
}

type EventBlock struct {
	Summary     string
	Description string
	Status      string
	Due         *time.Time
	Index       int
}

func NewEventBlock() *EventBlock {
	block := EventBlock{}
	block.Status = "NEEDS-ACTION"
	block.Index = -1
	block.Due = &time.Time{}
	return &block
}

func (e *EventBlock) ParseTitleLine(line string) error {
	headMatch := headersRegex.FindStringSubmatch(line)
	if len(headMatch) == 0 {
		return fmt.Errorf("No matches")
	}

	if headMatch[1] == "✔️" {
		e.Status = "COMPLETED"
	}
	e.Summary = headMatch[2]
	idx, err := strconv.Atoi(headMatch[3])
	if err != nil {
		return nil
	}

	e.Index = idx
	return nil
}

func (e *EventBlock) ParseDateLine(line string) error {
	if len(line) == 0 || line[0] != '@' {
		return fmt.Errorf("Not a date line")
	}

	t, err := time.ParseInLocation("2006-01-02 15:04", line[1:], time.Local)
	if err == nil {
		e.Due = &t
		return nil
	}
	t, err = naturaldate.Parse(string(line[1:]), time.Now(), naturaldate.WithDirection(naturaldate.Future))
	if err == nil {
		e.Due = &t
		return nil
	}
	return err
}

func WriteEvents(icalDir, fileName string, existingEvents []*ical.Component) error {
	contents, err := os.Open(fileName)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(contents)

	eventblock := NewEventBlock()
	eventBlocks := make([]*EventBlock, 0)
	for scanner.Scan() {
		line := scanner.Text()

		if len(line) > 0 && line[0] == '#' && eventblock.Summary != "" {
			eventBlocks = append(eventBlocks, eventblock)
			eventblock = NewEventBlock()
		}

		if err := eventblock.ParseTitleLine(line); err != nil {
			if err := eventblock.ParseDateLine(line); err != nil {
				eventblock.Description += line + "\n"
			}
			continue
		}
	}

	if eventblock.Summary != "" {
		eventBlocks = append(eventBlocks, eventblock)
	}

	for _, event := range eventBlocks {
		var component *ical.Component
		if event.Index == -1 || event.Index >= len(existingEvents) {
			component = createEvent()
		} else {
			component = existingEvents[event.Index]
		}

		fileNameProp := component.Props.Get("_FILENAME")
		var fileName string
		if fileNameProp == nil {
			fileName = component.Props.Get(ical.PropUID).Value + ".ics"
		} else {
			fileName = fileNameProp.Value
		}
		component.Props.Del("_FILENAME")
		data := encodeEvent(component, event)
		filePath := icalDir + fileName
		err := os.WriteFile(filePath, data, 0644)
		if err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}
