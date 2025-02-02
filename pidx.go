package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"sync"
)

//
//  This file contains all available packages from
//  all vendors.
//
type PidxXML struct {
	XMLName   xml.Name `xml:"index"`
	Timestamp string   `xml:"timestamp"`

	Pindex struct {
		XMLName xml.Name  `xml:"pindex"`
		Pdscs   []PdscTag `xml:"pdsc"`
	} `xml:"pindex"`

	pdscList map[string]bool
	force    bool
}

type PdscTag struct {
	XMLName   xml.Name `xml:"pdsc"`
	Vendor    string   `xml:"vendor,attr"`
	URL       string   `xml:"url,attr"`
	Name      string   `xml:"name,attr"`
	Version   string   `xml:"version,attr"`
	Timestamp string   `xml:"timestamp,attr"`
}

func NewPidx() *PidxXML {
	p := new(PidxXML)
	p.pdscList = make(map[string]bool)
	return p
}

func (p *PidxXML) addPdsc(pdsc PdscTag) error {
	pdscURL := pdsc.getURL()
	if p.pdscList[pdscURL] {
		message := fmt.Sprintf("Package %s/%s/%s already exists!", pdsc.Vendor, pdsc.Name, pdsc.Version)
		return errors.New(message)
	}

	if p.force {
		// The pdsc info in the tag should be ignored
		// and the actual pdsc is retrieved to get info cross-checked

		incomingPdscXML := new(PdscXML)
		if err := ReadXML(pdscURL, &incomingPdscXML); err != nil {
			// If it can't get the pdsc file, consider the pdsc tag to be valid
			p.Pindex.Pdscs = append(p.Pindex.Pdscs, pdsc)
			p.pdscList[pdscURL] = true

			return err
		}

		// Validate tag against the actual pdsc file
		if err := incomingPdscXML.MatchTag(pdsc); err != nil {
			// Prioritize information from pdsc file rather than tag
			correctPdscTag := incomingPdscXML.Tag()
			p.Pindex.Pdscs = append(p.Pindex.Pdscs, correctPdscTag)

			// Mark both wrong and correct pdsc in pdscList
			// to avoid duplication
			p.pdscList[pdscURL] = true
			p.pdscList[correctPdscTag.getURL()] = true

			return err
		}
	}

	p.Pindex.Pdscs = append(p.Pindex.Pdscs, pdsc)
	p.pdscList[pdscURL] = true
	return nil
}

func (p *PidxXML) ListPdsc() []PdscTag {
	Logger.Debug("Listing available packages")
	return p.Pindex.Pdscs
}

func updatePdscListTask(id int, vendorPidx VendorPidx, pidx *PidxXML, wg *sync.WaitGroup, err *error) {
	defer wg.Done()

	url := vendorPidx.URL + vendorPidx.Vendor + ".pidx"
	Logger.Info("[%d] Fetching packages list from %s", id, url)

	incomingPidx := new(PidxXML)
	if *err = ReadXML(url, &incomingPidx); *err != nil {
		return
	}

	Logger.Info("Adding pdscs")
	for _, pdsc := range incomingPidx.ListPdsc() {
		if *err = pidx.addPdsc(pdsc); *err != nil {
			return
		}
	}
}

func (p *PidxXML) Update(vidx *VidxXML) error {
	Logger.Info("Updating list of packages")

	var wg sync.WaitGroup

	// Process package index first
	errs := make([]error, vidx.PidxLength()+vidx.PdscLength())
	for i, vendorPidx := range vidx.ListPidx() {
		wg.Add(1)
		go updatePdscListTask(i, vendorPidx, p, &wg, &errs[i])
	}

	wg.Wait()

	// Now process package descriptors (vendors without pidx files)
	offset := vidx.PidxLength()
	for i, pdsc := range vidx.ListPdsc() {
		errs[i+offset] = p.addPdsc(pdsc)
	}

	if err := AnyErr(errs); err != nil {
		return err
	}

	return nil
}

func (p *PidxXML) SetForce(force bool) {
	p.force = force
}

func (p *PdscTag) getURL() string {
	return p.URL + p.Vendor + "." + p.Name + ".pdsc"
}
