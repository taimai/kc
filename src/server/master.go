package server

import (
	. "config"
	"fmt"
	"github.com/mitchellh/goamz/aws"
	"github.com/segmentio/go-route53"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	// k8s
	"flag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ELB States
const (
	CREATING = iota
	INUSE
	DUPLICATE
	ERROR
)

// ELB Info
type eLB struct {
	Name    string
	State   int
	Service string
	Error   string
}

// The environment
var env string
var region string

// Map to keep track of CNAMEs and their corresponding ELBs
var cNameInfo map[string]*eLB

// Map to keep track of CNAMEs with substituted env values
var cNameSubs map[string]string

// Dns client to query info about an ELB
var dns *route53.Client

var clientset *kubernetes.Clientset

func logError(err error) {
	log.Printf("** ERROR: %v\n", err)
}

// Error handler. Only logs the error for now
func check(err error) {
	if err != nil {
		logError(err)
	}
}

func init() {
	cNameInfo = make(map[string]*eLB)
	cNameSubs = make(map[string]string)

	env = os.Getenv("ENV")
	if env == "" {
		panic("ENV is not set")
	}
	log.Printf("ENV:%s\n", env)
	region = os.Getenv("REGION")
	if region == "" {
		panic("REGION is not set")
	}
	log.Printf("REGION:%s\n", region)

	auth, err := aws.EnvAuth()
	check(err)

	dns = route53.New(auth, aws.USWest)
}

// Connect from inside the k8s network
func connectFromInternal() {
	// Only create clientset once
	if clientset == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
		// Use insecure connection
		config.TLSClientConfig.Insecure = true
		config.TLSClientConfig.CAFile = ""
		config.TLSClientConfig.CertFile = ""
		config.TLSClientConfig.KeyFile = ""
		// Create the clientset
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
		log.Printf("Connected to %s\n", config.Host)
	}
}

// Connect to the k8s master from the outside network
func connectFromExternal() {
	// Only create clientset once
	if clientset == nil {
		kubeconfig := flag.String("kubeconfig", Config.K8sConfig, "absolute path to the kubeconfig file")
		flag.Parse()
		// Use the current context in kubeconfig
		config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}
		// Create the clientset
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			panic(err.Error())
		}
		log.Printf("Connected to %s\n", config.Host)
	}
}

func Setup() {
	// See if we're connecting to the cluster externally or internally
	if Config.K8sConfig != "" {
		connectFromExternal()
	} else {
		connectFromInternal()
	}
}

// Connect to the k8s master and return a list of services
func GetServices() {
	if clientset == nil {
		return
	}

	services, err := clientset.CoreV1().Services("").List(metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	log.Printf("Scanning list of %d services in cluster\n", len(services.Items))
	for _, service := range services.Items {
		//fmt.Printf("%+v\n", service)
		cname := service.ObjectMeta.Annotations["domainName"]
		if cname == "" || service.Status.LoadBalancer.Ingress == nil {
			continue
		}

		// Do the env and region substitutions, if any. Save the results.
		cnameRaw := cname
		if cNameSubs[cnameRaw] != "" {
			cname = cNameSubs[cnameRaw]
		} else {
			if strings.Contains(cnameRaw, "{env}") {
				cname = strings.Replace(cnameRaw, "{env}", env, -1)
			}
			if strings.Contains(cname, "{region}") {
				cname = strings.Replace(cname, "{region}", region, -1)
			}
			if cnameRaw != cname && cNameInfo[cname] == nil {
				log.Printf("!! CNAME: %s --> %s\n", cnameRaw, cname)
			}
			cNameSubs[cnameRaw] = cname
		}

		serviceName := service.ObjectMeta.Name
		elbName := service.Status.LoadBalancer.Ingress[0].Hostname
		//log.Printf("%s %s %s\n", serviceName, cname, elbName)

		res, err := dns.Zone(Config.HostedZoneID).RecordsByName(cname)
		check(err)
		if len(res) == 0 {
			// Check if we need to create a CNAME
			if cNameInfo[cname] == nil {
				log.Printf("CNAME %s doesn't exist. Creating...\n", cname)
				res, err := dns.Zone(Config.HostedZoneID).Add("CNAME", cname, elbName)
				check(err)
				log.Println(res)
				if err == nil {
					cNameInfo[cname] = &eLB{elbName, CREATING, serviceName, ""}
					log.Printf("++ CNAME %s created for service %s\n", cname, serviceName)
				}
			}
		} else {
			for _, record := range res {
				//fmt.Println(record.Name) // record.Name is cname
				elbName := record.Records[0] // this is the elb address
				if cNameInfo[cname] == nil {
					cNameInfo[cname] = &eLB{elbName, INUSE, serviceName, ""}
					log.Printf("Service %s: CNAME %s added to list\n", serviceName, cname)
				}
				break
			}
		}
	}

	checkELBs()
	log.Println()
}

func ClearServices() {
	cNameInfo = nil
	cNameSubs = nil
	cNameInfo = make(map[string]*eLB)
	cNameSubs = make(map[string]string)
}

// Scan and delete CNAMEs if their corresponding ELBs have been deleted
func checkELBs() {
	for cname, elb := range cNameInfo {
		if elb.Name == "" {
			continue
		}
		log.Printf("Checking elb for cname %s\n", cname)
		_, err := net.LookupHost(elb.Name)
		check(err)
		if err != nil {
			if elb.State == CREATING {
				log.Printf("## ELB for cname %s is still not up yet\n", cname)
				continue
			}
			log.Printf("Elb doesn't exist. Deleting cname %s...\n", cname)
			// Remove CNAME
			res, err := dns.Zone(Config.HostedZoneID).Remove("CNAME", cname, elb.Name)
			check(err)
			log.Println(res)
			if err == nil {
				cNameInfo[cname].Name = ""
				delete(cNameInfo, cname)
			}
		} else if cNameInfo[cname].State == CREATING {
			log.Printf(">> ELB for cname %s is now up\n", cname)
			cNameInfo[cname].State = INUSE
		}
	}
}

//
// Handlers and their support functions
//
func listCnames() string {
	var list string
	for cname, elbInfo := range cNameInfo {
		if elbInfo.Name == "" {
			continue
		}
		list += fmt.Sprintf("service=%s cname=%s elb=%s state=%d\n",
			elbInfo.Service, cname, elbInfo.Name, elbInfo.State)
	}

	return list
}

func ListCnamesHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(listCnames()))
}
