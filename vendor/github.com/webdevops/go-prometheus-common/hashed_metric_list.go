package prometheus_common

import (
	"crypto/sha256"
	"fmt"
	"github.com/patrickmn/go-cache"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

type HashedMetricList struct {
	list map[string]*MetricRow
	mux  *sync.Mutex

	metricsCache *cache.Cache
}

func NewHashedMetricsList() *HashedMetricList {
	m := HashedMetricList{}
	m.Init()
	return &m
}

func (m *HashedMetricList) Init() {
	m.mux = &sync.Mutex{}
	m.Reset()
}

func (m *HashedMetricList) SetCache(instance *cache.Cache) {
	m.metricsCache = instance
}

func (m *HashedMetricList) LoadFromCache(key string) bool {
	m.Reset()

	if m.metricsCache != nil {
		m.mux.Lock()
		defer m.mux.Unlock()

		if val, fetched := m.metricsCache.Get(key); fetched {
			// loaded from cache
			m.list = val.(map[string]*MetricRow)
			return true
		}
	}

	return false
}

func (m *HashedMetricList) StoreToCache(key string, duration time.Duration) {
	if m.metricsCache != nil {
		m.metricsCache.Add(key, m.GetList(), duration)
	}
}

func (m *HashedMetricList) Reset() {
	m.mux.Lock()
	defer m.mux.Unlock()
	m.list = map[string]*MetricRow{}
}

func (m *HashedMetricList) GetList() []MetricRow {
	m.mux.Lock()
	defer m.mux.Unlock()

	list := []MetricRow{}
	for _, row := range m.list {
		list = append(list, *row)
	}

	return list
}

func (m *HashedMetricList) Inc(labels prometheus.Labels) {
	m.mux.Lock()
	defer m.mux.Unlock()

	metricKey := ""
	for key, value := range labels {
		metricKey = metricKey + key + "=" + value + ";"
	}
	hashKey := fmt.Sprintf("%x", sha256.Sum256([]byte(metricKey)))
	if _, exists := m.list[hashKey]; exists {
		m.list[hashKey].value++
	} else {
		m.list[hashKey] = &MetricRow{
			labels: labels,
			value:  1,
		}
	}
}

func (m *HashedMetricList) GaugeSet(gauge *prometheus.GaugeVec) {
	for _, metric := range m.GetList() {
		gauge.With(metric.labels).Set(metric.value)
	}
}

func (m *HashedMetricList) CounterAdd(counter *prometheus.CounterVec) {
	for _, metric := range m.GetList() {
		counter.With(metric.labels).Add(metric.value)
	}
}
