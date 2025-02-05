/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package uszipam

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const (
	priority    = 10 // default priority when nothing is set
	etcdTimeout = 5 * time.Second

	randomPrefixLabel = "uszipam"
)

// uszIpamClient is an interface to work with UszIpam service records in etcd
type uszIpamClient interface {
	GetServices(prefix string) ([]*Service, error)
	SaveService(value *Service) error
	DeleteService(key string) error
}

type uszIpamProvider struct {
	provider.BaseProvider
	dryRun        bool
	uszIpamPrefix string
	domainFilter  endpoint.DomainFilter
	client        uszIpamClient
}

// Service represents UszIpam record
type Service struct {
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Weight   int    `json:"weight,omitempty"`
	Text     string `json:"text,omitempty"`
	Mail     bool   `json:"mail,omitempty"` // Be an MX record. Priority becomes Preference.
	TTL      uint32 `json:"ttl,omitempty"`

	// When a SRV record with a "Host: IP-address" is added, we synthesize
	// a srv.Target domain name.  Normally we convert the full Key where
	// the record lives to a DNS name and use this as the srv.Target.  When
	// TargetStrip > 0 we strip the left most TargetStrip labels from the
	// DNS name.
	TargetStrip int `json:"targetstrip,omitempty"`

	// Group is used to group (or *not* to group) different services
	// together. Services with an identical Group are returned in the same
	// answer.
	Group string `json:"group,omitempty"`

	// Etcd key where we found this service and ignored from json un-/marshaling
	Key string `json:"-"`
}

type ipamclient struct {
	ctx    context.Context
	config *Config
}

var _ uszIpamClient = ipamclient{}

// GetService return all Service records stored in bluecat stored anywhere under the given key (recursively)
func (c ipamclient) GetServices(prefix string) ([]*Service, error) {
	ctx, cancel := context.WithTimeout(c.ctx, etcdTimeout)
	defer cancel()

	log.WithFields(
		log.Fields{
			"Provider": "UszIpam",
			"LOG_ID":   "WEB-HhRNi",
		}).Println(prefix, ctx)

	return []*Service{}, nil
}

// SaveService persists service data into rest api
func (c ipamclient) SaveService(service *Service) error {
	ctx, cancel := context.WithTimeout(c.ctx, etcdTimeout)
	defer cancel()

	value, err := json.Marshal(&service)
	if err != nil {
		return err
	}

	// TODO: send data to rest api
	log.WithFields(
		log.Fields{
			"Provider": "UszIpam",
			"LOG_ID":   "WEB-SKpPn",
		}).Println(string(value), ctx)

	return nil
}

// DeleteService deletes service record from etcd
func (c ipamclient) DeleteService(key string) error {
	ctx, cancel := context.WithTimeout(c.ctx, etcdTimeout)
	defer cancel()

	// TODO: send data to rest api
	log.WithFields(
		log.Fields{
			"Provider": "UszIpam",
			"LOG_ID":   "WEB-myHiQ",
		}).Println(key, ctx)
	return nil
}

// newUszIpamClient is an etcd client constructor
func newUszIpamClient() (uszIpamClient, error) {
	cfg, err := NewConfig()
	if err != nil {
		return nil, err
	}
	log.WithFields(
		log.Fields{
			"Provider": "UszIpam",
			"LOG_ID":   "WEB-E09A0",
		}).Println(cfg)
	return ipamclient{context.Background(), cfg}, nil
}

// NewUszIpamProvider is a UszIpam provider constructor
func NewUszIpamProvider(domainFilter endpoint.DomainFilter, prefix string, dryRun bool) (provider.Provider, error) {
	client, err := newUszIpamClient()
	if err != nil {
		return nil, err
	}

	return uszIpamProvider{
		client:        client,
		dryRun:        dryRun,
		uszIpamPrefix: prefix,
		domainFilter:  domainFilter,
	}, nil
}

// findEp takes an Endpoint slice and looks for an element in it. If found it will
// return Endpoint, otherwise it will return nil and a bool of false.
func findEp(slice []*endpoint.Endpoint, dnsName string) (*endpoint.Endpoint, bool) {
	for _, item := range slice {
		if item.DNSName == dnsName {
			return item, true
		}
	}
	return nil, false
}

// findLabelInTargets takes an ep.Targets string slice and looks for an element in it. If found it will
// return its string value, otherwise it will return empty string and a bool of false.
func findLabelInTargets(targets []string, label string) (string, bool) {
	for _, target := range targets {
		if target == label {
			return target, true
		}
	}
	return "", false
}

// SupportedRecordType returns true if the record type is supported by the provider
func (p *uszIpamProvider) SupportedRecordType(recordType string) bool {
	switch recordType {
	case "A", "AAAA", "CNAME", "SRV", "TXT", "NS":
		return true
	default:
		return false
	}
}

// Records returns all DNS records found in UszIpam backend. Depending on the record fields
// it may be mapped to one or two records of type A, CNAME, TXT, A+TXT, CNAME+TXT
func (p uszIpamProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	var result []*endpoint.Endpoint
	services, err := p.client.GetServices(p.uszIpamPrefix)
	if err != nil {
		return nil, err
	}
	for _, service := range services {
		domains := strings.Split(strings.TrimPrefix(service.Key, p.uszIpamPrefix), "/")
		reverse(domains)
		dnsName := strings.Join(domains[service.TargetStrip:], ".")
		if !p.domainFilter.Match(dnsName) {
			continue
		}
		log.WithFields(
			log.Fields{
				"Provider": "UszIpam",
				"LOG_ID":   "WEB-UtH8m",
			}).Debugf("Getting service (%v) with service host (%s)", service, service.Host)
		prefix := strings.Join(domains[:service.TargetStrip], ".")
		if service.Host != "" {
			ep, found := findEp(result, dnsName)
			if found {
				ep.Targets = append(ep.Targets, service.Host)
				log.WithFields(
					log.Fields{
						"Provider": "UszIpam",
						"LOG_ID":   "WEB-dzyjC",
					}).Debugf("Extending ep (%s) with new service host (%s)", ep, service.Host)
			} else {
				ep = endpoint.NewEndpointWithTTL(
					dnsName,
					guessRecordType(service.Host),
					endpoint.TTL(service.TTL),
					service.Host,
				)
				log.WithFields(
					log.Fields{
						"Provider": "UszIpam",
						"LOG_ID":   "WEB-ayWRZ",
					}).Debugf("Creating new ep (%s) with new service host (%s)", ep, service.Host)
			}
			ep.Labels["originalText"] = service.Text
			ep.Labels[randomPrefixLabel] = prefix
			ep.Labels[service.Host] = prefix
			result = append(result, ep)
		}
		if service.Text != "" {
			ep := endpoint.NewEndpoint(
				dnsName,
				endpoint.RecordTypeTXT,
				service.Text,
			)
			ep.Labels[randomPrefixLabel] = prefix
			result = append(result, ep)
		}
	}
	return result, nil
}

// ApplyChanges stores changes back to etcd converting them to UszIpam format and aggregating A/CNAME and TXT records
func (p uszIpamProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	grouped := map[string][]*endpoint.Endpoint{}
	for _, ep := range changes.Create {
		grouped[ep.DNSName] = append(grouped[ep.DNSName], ep)
	}
	for i, ep := range changes.UpdateNew {
		ep.Labels = changes.UpdateOld[i].Labels
		log.WithFields(
			log.Fields{
				"Provider": "UszIpam",
				"LOG_ID":   "WEB-eD0ao",
			}).Debugf("Updating labels (%s) with old labels(%s)", ep.Labels, changes.UpdateOld[i].Labels)
		grouped[ep.DNSName] = append(grouped[ep.DNSName], ep)
	}
	for dnsName, group := range grouped {
		if !p.domainFilter.Match(dnsName) {
			log.WithFields(
				log.Fields{
					"Provider": "UszIpam",
					"LOG_ID":   "WEB-qQPj0",
				}).Debugf("Skipping record %s because it was filtered out by the specified --domain-filter", dnsName)
			continue
		}
		var services []Service
		for _, ep := range group {
			if ep.RecordType == endpoint.RecordTypeTXT {
				continue
			}

			for _, target := range ep.Targets {
				prefix := ep.Labels[target]
				log.WithFields(
					log.Fields{
						"Provider": "UszIpam",
						"LOG_ID":   "WEB-gLxuh",
					}).Debugf("Getting prefix(%s) from label(%s)", prefix, target)
				if prefix == "" {
					prefix = fmt.Sprintf("%08x", rand.Int31())
					log.WithFields(
						log.Fields{
							"Provider": "UszIpam",
							"LOG_ID":   "WEB-R9osw",
						}).Infof("Generating new prefix: (%s)", prefix)
				}

				service := Service{
					Host:        target,
					Text:        ep.Labels["originalText"],
					Key:         p.keyFor(prefix + "." + dnsName),
					TargetStrip: strings.Count(prefix, ".") + 1,
					TTL:         uint32(ep.RecordTTL),
				}
				services = append(services, service)
				ep.Labels[target] = prefix
				log.WithFields(
					log.Fields{
						"Provider": "UszIpam",
						"LOG_ID":   "WEB-aOdbZ",
					}).Debugf("Putting prefix(%s) to label(%s)", prefix, target)
				log.WithFields(
					log.Fields{
						"Provider": "UszIpam",
						"LOG_ID":   "WEB-3S1bm",
					}).Debugf("Ep labels structure now: (%v)", ep.Labels)
			}

			// Clean outdated targets
			for label, labelPrefix := range ep.Labels {
				// Skip non Target related labels
				labelsToSkip := []string{"originalText", "prefix", "resource"}
				if _, ok := findLabelInTargets(labelsToSkip, label); ok {
					continue
				}

				log.WithFields(
					log.Fields{
						"Provider": "UszIpam",
						"LOG_ID":   "WEB-FJcPz",
					}).Debugf("Finding label (%s) in targets(%v)", label, ep.Targets)
				if _, ok := findLabelInTargets(ep.Targets, label); !ok {
					log.WithFields(
						log.Fields{
							"Provider": "UszIpam",
							"LOG_ID":   "WEB-Vq1Z9",
						}).Debugf("Found non existing label(%s) in targets(%v)", label, ep.Targets)
					dnsName := ep.DNSName
					dnsName = labelPrefix + "." + dnsName
					key := p.keyFor(dnsName)
					log.WithFields(
						log.Fields{
							"Provider": "UszIpam",
							"LOG_ID":   "WEB-e1lB6",
						}).Infof("Delete key %s", key)
					if !p.dryRun {
						err := p.client.DeleteService(key)
						if err != nil {
							return err
						}
					}
				}
			}
		}
		index := 0
		for _, ep := range group {
			if ep.RecordType != endpoint.RecordTypeTXT {
				continue
			}
			if index >= len(services) {
				prefix := ep.Labels[randomPrefixLabel]
				if prefix == "" {
					prefix = fmt.Sprintf("%08x", rand.Int31())
				}
				services = append(services, Service{
					Key:         p.keyFor(prefix + "." + dnsName),
					TargetStrip: strings.Count(prefix, ".") + 1,
					TTL:         uint32(ep.RecordTTL),
				})
			}
			services[index].Text = ep.Targets[0]
			index++
		}

		for i := index; index > 0 && i < len(services); i++ {
			services[i].Text = ""
		}

		for _, service := range services {
			log.WithFields(
				log.Fields{
					"Provider": "UszIpam",
					"LOG_ID":   "WEB-x2tYe",
				}).Infof("Add/set key %s to Host=%s, Text=%s, TTL=%d", service.Key, service.Host, service.Text, service.TTL)
			if !p.dryRun {
				err := p.client.SaveService(&service)
				if err != nil {
					return err
				}
			}
		}
	}

	for _, ep := range changes.Delete {
		dnsName := ep.DNSName
		if ep.Labels[randomPrefixLabel] != "" {
			dnsName = ep.Labels[randomPrefixLabel] + "." + dnsName
		}
		key := p.keyFor(dnsName)
		log.WithFields(
			log.Fields{
				"Provider": "UszIpam",
				"LOG_ID":   "WEB-OKvcp",
			}).Infof("Delete key %s", key)
		if !p.dryRun {
			err := p.client.DeleteService(key)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (p uszIpamProvider) keyFor(dnsName string) string {
	domains := strings.Split(dnsName, ".")
	reverse(domains)
	log.WithFields(
		log.Fields{
			"Provider": "UszIpam",
			"LOG_ID":   "WEB-FazCK",
		}).Infof("keyFor  %s", domains)

	return p.uszIpamPrefix + strings.Join(domains, "/")
}

func guessRecordType(target string) string {
	if net.ParseIP(target) != nil {
		return endpoint.RecordTypeA
	}
	return endpoint.RecordTypeCNAME
}

func reverse(slice []string) {
	for i := 0; i < len(slice)/2; i++ {
		j := len(slice) - i - 1
		slice[i], slice[j] = slice[j], slice[i]
	}
}
