package main

import (
	"os"
	"fmt"
	"time"
	"net/http"
	"github.com/jessevdk/go-flags"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	AzureDevops "azure-devops-exporter/src/azure-devops-client"
)

const (
	Author  = "webdevops.io"
	Version = "0.3.1"
	AZURE_RESOURCEGROUP_TAG_PREFIX = "tag_"
)

var (
	argparser          *flags.Parser
	args               []string
	Logger             *DaemonLogger
	ErrorLogger        *DaemonLogger
	AzureDevopsClient  *AzureDevops.AzureDevopsClient

	collectorGeneralList    map[string]*CollectorGeneral
	collectorProjectList    map[string]*CollectorProject
	collectorAgentPoolList    map[string]*CollectorAgentPool
)

var opts struct {
	// general settings
	Verbose     []bool `            long:"verbose" short:"v"                    env:"VERBOSE"                       description:"Verbose mode"`

	// server settings
	ServerBind  string `            long:"bind"                                 env:"SERVER_BIND"                   description:"Server address"                                    default:":8080"`

	// scrape time settings
	ScrapeTime  time.Duration `            long:"scrape-time"                  env:"SCRAPE_TIME"                    description:"Scrape time (time.duration)"                       default:"15m"`
	ScrapeTimeGeneral  *time.Duration `    long:"scrape-time-general"          env:"SCRAPE_TIME_GENERAL"            description:"Scrape time for general metrics (time.duration)"   default:"15s"`
	ScrapeTimeBuild  *time.Duration `      long:"scrape-time-build"            env:"SCRAPE_TIME_BUILD"              description:"Scrape time for general metrics (time.duration)"`
	ScrapeTimeRelease  *time.Duration `    long:"scrape-time-release"          env:"SCRAPE_TIME_RELEASE"            description:"Scrape time for general metrics (time.duration)"`
	ScrapeTimePullRequest *time.Duration ` long:"scrape-time-pullrequest"      env:"SCRAPE_TIME_PULLREQUEST"        description:"Scrape time for quota metrics  (time.duration)"`
	ScrapeTimeLatestBuild  *time.Duration `long:"scrape-time-latest-build"     env:"SCRAPE_TIME_LATEST_BUILD"       description:"Scrape time for general metrics (time.duration)"   default:"30s"`
	ScrapeTimeAgentPool *time.Duration `   long:"scrape-time-agentpool"        env:"SCRAPE_TIME_AGENTPOOL"          description:"Scrape time for agent queues (time.duration)"      default:"30s"`

	// azure settings
	AzureDevopsAccessToken string ` long:"azure-devops-access-token"            env:"AZURE_DEVOPS_ACCESS_TOKEN"                      description:"Azure DevOps access token" required:"true"`
	AzureDevopsOrganisation string `long:"azure-devops-organisation"            env:"AZURE_DEVOPS_ORGANISATION"                      description:"Azure DevOps organization" required:"true"`
	AzureDevopsFilterAgentPoolId []int64 `long:"azure-devops-filter-agentpool"  env:"AZURE_DEVOPS_FILTER_AGENTPOOL"  env-delim:" "   description:"Filter of agent pool (IDs)"`
}

func main() {
	initArgparser()

	Logger = CreateDaemonLogger(0)
	ErrorLogger = CreateDaemonErrorLogger(0)

	// set verbosity
	Verbose = len(opts.Verbose) >= 1

	Logger.Messsage("Init Azure DevOps exporter v%s (written by %v)", Version, Author)

	Logger.Messsage("Init Azure connection")
	initAzureConnection()

	Logger.Messsage("Starting metrics collection")
	Logger.Messsage("  scape time: %v", opts.ScrapeTime)
	initMetricCollector()

	Logger.Messsage("Starting http server on %s", opts.ServerBind)
	startHttpServer()
}

// init argparser and parse/validate arguments
func initArgparser() {
	argparser = flags.NewParser(&opts, flags.Default)
	_, err := argparser.Parse()

	// check if there is an parse error
	if err != nil {
		if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		} else {
			fmt.Println(err)
			fmt.Println()
			argparser.WriteHelp(os.Stdout)
			os.Exit(1)
		}
	}

	// scrape time
	if opts.ScrapeTimeGeneral == nil {
		opts.ScrapeTimeGeneral = &opts.ScrapeTime
	}

	if opts.ScrapeTimePullRequest == nil {
		opts.ScrapeTimePullRequest = &opts.ScrapeTime
	}

	if opts.ScrapeTimeBuild == nil {
		opts.ScrapeTimeBuild = &opts.ScrapeTime
	}

	if opts.ScrapeTimeRelease == nil {
		opts.ScrapeTimeRelease = &opts.ScrapeTime
	}

	if opts.ScrapeTimeAgentPool == nil {
		opts.ScrapeTimeAgentPool = &opts.ScrapeTime
	}

	if opts.ScrapeTimeLatestBuild == nil {
		opts.ScrapeTimeLatestBuild = &opts.ScrapeTime
	}
}

// Init and build Azure authorzier
func initAzureConnection() {
	AzureDevopsClient = AzureDevops.NewAzureDevopsClient()
	AzureDevopsClient.SetOrganization(opts.AzureDevopsOrganisation)
	AzureDevopsClient.SetAccessToken(opts.AzureDevopsAccessToken)
}

func initMetricCollector() {
	var collectorName string
	collectorGeneralList = map[string]*CollectorGeneral{}
	collectorProjectList = map[string]*CollectorProject{}
	collectorAgentPoolList = map[string]*CollectorAgentPool{}

	projectList, err := AzureDevopsClient.ListProjects()

	if err != nil {
		panic(err)
	}

	collectorName = "General"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorGeneralList[collectorName] = NewCollectorGeneral(collectorName, &MetricsCollectorGeneral{})
		collectorGeneralList[collectorName].AzureDevOpsProjects = &projectList
		collectorGeneralList[collectorName].Run(*opts.ScrapeTimeGeneral)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Project"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorProjectList[collectorName] = NewCollectorProject(collectorName, &MetricsCollectorProject{})
		collectorProjectList[collectorName].AzureDevOpsProjects = &projectList
		collectorProjectList[collectorName].Run(*opts.ScrapeTimeGeneral)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "PullRequest"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorProjectList[collectorName] = NewCollectorProject(collectorName, &MetricsCollectorPullRequest{})
		collectorProjectList[collectorName].AzureDevOpsProjects = &projectList
		collectorProjectList[collectorName].Run(*opts.ScrapeTimePullRequest)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "LatestBuild"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorProjectList[collectorName] = NewCollectorProject(collectorName, &MetricsCollectorLatestBuild{})
		collectorProjectList[collectorName].AzureDevOpsProjects = &projectList
		collectorProjectList[collectorName].Run(*opts.ScrapeTimeLatestBuild)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Build"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorProjectList[collectorName] = NewCollectorProject(collectorName, &MetricsCollectorBuild{})
		collectorProjectList[collectorName].AzureDevOpsProjects = &projectList
		collectorProjectList[collectorName].Run(*opts.ScrapeTimeBuild)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "Release"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorProjectList[collectorName] = NewCollectorProject(collectorName, &MetricsCollectorRelease{})
		collectorProjectList[collectorName].AzureDevOpsProjects = &projectList
		collectorProjectList[collectorName].Run(*opts.ScrapeTimeRelease)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}

	collectorName = "AgentPool"
	if opts.ScrapeTimeGeneral.Seconds() > 0 {
		collectorAgentPoolList[collectorName] = NewCollectorAgentPool(collectorName, &MetricsCollectorAgentPool{})
		collectorAgentPoolList[collectorName].AzureDevOpsProjects = &projectList
		collectorAgentPoolList[collectorName].AgentPoolIdList = opts.AzureDevopsFilterAgentPoolId
		collectorAgentPoolList[collectorName].Run(*opts.ScrapeTimeAgentPool)
	} else {
		Logger.Messsage("collector[%s]: disabled", collectorName)
	}
}

// start and handle prometheus handler
func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	ErrorLogger.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}
