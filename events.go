package crapdav

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"sort"

	"github.com/emersion/go-ical"
)

func ParseEvents(icalDir string) (error, []*ical.Component) {

	events := make([]*ical.Component, 0)

	files, err := ioutil.ReadDir(icalDir)
	if err != nil {
		return err, []*ical.Component{}
	}

	for _, f := range files {
		r, err := os.Open(icalDir + f.Name())
		if err != nil {
			return err, []*ical.Component{}
		}

		dec := ical.NewDecoder(r)
		for {
			cal, err := dec.Decode()
			if err == io.EOF {
				break
			} else if err != nil {
				log.Fatal(err)
			}

			for _, child := range cal.Children {
				if child.Name == ical.CompToDo {
					prop := ical.NewProp("_FILENAME")
					prop.SetText(f.Name())
					child.Props.Add(prop)
					events = append(events, child)
				}
			}
		}
	}

	sort.Sort(byEvents(events))

	return nil, events
}

type byEvents []*ical.Component

func (s byEvents) Len() int {
	return len(s)
}
func (s byEvents) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func Get(component *ical.Component, name string) string {
	prop := component.Props.Get(name)
	if prop == nil {
		return ""
	}
	return prop.Value
}

func GetText(component *ical.Component, name string) string {
	v, err := component.Props.Text(name)
	if err != nil {
		return ""
	}
	return v
}

func (s byEvents) Less(i, j int) bool {
	a := s[i]
	b := s[j]
	aDue := Get(a, ical.PropDue)
	bDue := Get(b, ical.PropDue)

	aCreated := Get(a, ical.PropDateTimeStamp)
	bCreated := Get(b, ical.PropDateTimeStamp)

	aStatus := Get(a, ical.PropStatus)
	bStatus := Get(b, ical.PropStatus)

	if aStatus == bStatus {
		if aDue == bDue {
			return aCreated < bCreated
		}

		if aDue == "" {
			return false
		}
		if bDue == "" {
			return true
		}
		return aDue < bDue
	}

	if aStatus == "COMPLETED" {
		return false
	}
	if bStatus == "COMPLETED" {
		return true
	}
	return aStatus < bStatus
}
