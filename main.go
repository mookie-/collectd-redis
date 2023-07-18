package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis"
)

const version = "v0.1.2"

var collectdInterval = getCollectdInterval()
var hostname = getHostname()

// for compatibility with the collectd redis plugin (https://collectd.org/wiki/index.php/Plugin:Redis)
// we need to map some redis metric names to the plugin specific names written to the time series database
var uniqueMetricMap = map[string]string{
	"blocked_clients":             "blocked_clients",
	"connected_clients":           "current_connections-clients",
	"connected_slaves":            "current_connections-slaves",
	"evicted_keys":                "evicted_keys",
	"expired_keys":                "expired_keys",
	"keyspace_hits":               "cache_result-hits",
	"keyspace_misses":             "cache_result-misses",
	"pubsub_channels":             "pubsub-channels",
	"pubsub_patterns":             "pubsub-patterns",
	"rdb_changes_since_last_save": "volatile_changes",
	"total_commands_processed":    "total_operations",
	"total_connections_received":  "total_connections",
	"total_net_input_bytes":       "total_bytes-input",
	"total_net_output_bytes":      "total_bytes-output",
	"uptime_in_seconds":           "uptime",
	"used_cpu_sys_children":       "ps_cputime-children/syst",
	"used_cpu_sys":                "ps_cputime-daemon/syst",
	"used_cpu_user_children":      "ps_cputime-children/user",
	"used_cpu_user":               "ps_cputime-daemon/user",
	"used_memory_lua":             "memory_lua",
	"used_memory":                 "memory",
}

func errCheckFatal(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getHostname() string {
	host := os.Getenv("COLLECTD_HOSTNAME")
	if host == "" {
		host = "localhost"
	}
	return host
}

func getCollectdInterval() float64 {
	intervalFromEnv := os.Getenv("COLLECTD_INTERVAL")

	if intervalFromEnv == "" {
		intervalFromEnv = "10"
	}

	collectdInterval, err := strconv.ParseFloat(intervalFromEnv, 64)
	errCheckFatal(err)
	return collectdInterval
}

func redisMetrics(redisInstance redisInstance, redisClient *redis.Client) {
	info, err := fetchRedisInfo(redisClient)
	if err != nil {
		log.Printf("Error when trying to fetch info from redis instance <%s>.\n", redisInstance.name)
		log.Println(err)
	} else {
		var metrics []redisMetric
		metrics = append(metrics, generateUniqueMetrics(info)...)
		metrics = append(metrics, generateRecordsMetrics(info)...)
		for _, metric := range metrics {
			fmt.Print(parsePutvalString(redisInstance.name, metric))
		}
	}
}

func redisLatencyMetrics(redisInstance redisInstance, redisClient *redis.Client) {
  latency_events := []string{"command", "fast-command", "fork", "expire-cycle", "eviction-cycle", "eviction-del"}
  var metrics []redisMetric

  for i := 0; i < len(latency_events); i++ {
    latency_info, err := fetchRedisLatencyInfo(redisClient, latency_events[i])
    if err != nil {
      log.Printf("Error when trying to fetch latency info from redis instance <%s>.\n", redisInstance.name)
      log.Println(err)
    } else {
      metrics = append(metrics, generateLatencyMetrics(latency_info, latency_events[i])...)
    }
  }
  for _, metric := range metrics {
    fmt.Print(parsePutvalString(redisInstance.name, metric))
  }
}

func main() {
	args := os.Args[1:]
	if len(args) < 1 && len(args) > 2 {
		log.Fatal("The first argument needs to be the Redis URL <name>:<host>:<port>[:<password>] the second argument is optional and enables latency metrics.")
	}
	if args[0] == "version" {
		fmt.Println(version)
		os.Exit(0)
	}

	redisInstance := parseArgToInstance(args[0])
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisInstance.host, redisInstance.port),
		Password: redisInstance.pw,
	})

	for {
		redisMetrics(redisInstance, redisClient)
    if len(os.Args) == 3 {
      if os.Args[2] == "l" {
        redisLatencyMetrics(redisInstance, redisClient)
      } else {
        log.Fatal("The second argument needs to be 'l' to collectd latency metrics")
      }
    }
		time.Sleep(time.Duration(collectdInterval) * time.Second)
	}
}
