package llmbackend

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	servingSampleCeiling           = 100
	servingJudgementMinimumSamples = 5
	servingJudgementDelayPerMedian = 2
	slowServingThroughputFraction  = 0.5
	// OpenRouter ranks a model's providers over a rolling five-minute window.
	// https://openrouter.ai/docs/features/provider-routing
	servingCutMemory = 5 * time.Minute
)

type servingSample struct {
	Provider            string
	Duration            time.Duration
	CharactersPerSecond float64
}

type servingExpectation struct {
	MedianDuration            time.Duration
	MedianCharactersPerSecond float64
}

func (expectation servingExpectation) judgesSlow(elapsed time.Duration, outputCharacters int64) bool {
	if elapsed < expectation.MedianDuration*servingJudgementDelayPerMedian {
		return false
	}
	throughput := float64(outputCharacters) / elapsed.Seconds()
	return throughput < expectation.MedianCharactersPerSecond*slowServingThroughputFraction
}

type slowServingError struct {
	Provider            string
	Elapsed             time.Duration
	CharactersPerSecond float64
	Expectation         servingExpectation
}

func (failure slowServingError) Error() string {
	return fmt.Sprintf("%s served %.0f characters/s for %.1fs where the model's median is %.0f characters/s in %.1fs", firstNonEmpty(failure.Provider, "the provider"), failure.CharactersPerSecond, failure.Elapsed.Seconds(), failure.Expectation.MedianCharactersPerSecond, failure.Expectation.MedianDuration.Seconds())
}

type servingRecord struct {
	mutex          sync.Mutex
	samplesByModel map[string][]servingSample
	cutsByModel    map[string]map[string]time.Time
	now            func() time.Time
}

var sharedServingRecord = newServingRecord()

func newServingRecord() *servingRecord {
	return &servingRecord{samplesByModel: map[string][]servingSample{}, cutsByModel: map[string]map[string]time.Time{}, now: time.Now}
}

func (record *servingRecord) recordSample(modelName string, sample servingSample) {
	if modelName == "" || sample.Duration <= 0 {
		return
	}
	record.mutex.Lock()
	defer record.mutex.Unlock()
	samples := append(record.samplesByModel[modelName], sample)
	if len(samples) > servingSampleCeiling {
		samples = samples[len(samples)-servingSampleCeiling:]
	}
	record.samplesByModel[modelName] = samples
}

func (record *servingRecord) expectation(modelName string) (servingExpectation, bool) {
	record.mutex.Lock()
	defer record.mutex.Unlock()
	samples := record.samplesByModel[modelName]
	if len(samples) < servingJudgementMinimumSamples {
		return servingExpectation{}, false
	}
	durations := make([]float64, 0, len(samples))
	throughputs := make([]float64, 0, len(samples))
	for _, sample := range samples {
		durations = append(durations, sample.Duration.Seconds())
		throughputs = append(throughputs, sample.CharactersPerSecond)
	}
	return servingExpectation{
		MedianDuration:            time.Duration(medianFloat(durations) * float64(time.Second)),
		MedianCharactersPerSecond: medianFloat(throughputs),
	}, true
}

func (record *servingRecord) noteCut(modelName string, providerName string) {
	slug := providerSlug(providerName)
	if modelName == "" || slug == "" {
		return
	}
	record.mutex.Lock()
	defer record.mutex.Unlock()
	cuts := record.cutsByModel[modelName]
	if cuts == nil {
		cuts = map[string]time.Time{}
		record.cutsByModel[modelName] = cuts
	}
	cuts[slug] = record.now()
}

func (record *servingRecord) ignoredProviders(modelName string) []string {
	record.mutex.Lock()
	defer record.mutex.Unlock()
	now := record.now()
	slugs := []string{}
	for slug, cutAt := range record.cutsByModel[modelName] {
		if now.Sub(cutAt) > servingCutMemory {
			delete(record.cutsByModel[modelName], slug)
			continue
		}
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}

// OpenRouter names a provider in its response ("Sail Research", "Z.AI") and
// addresses it in routing by a slug ("sail-research", "z-ai"); the endpoint
// listing at /api/v1/models/{model}/endpoints carries both.
func providerSlug(providerName string) string {
	var slug strings.Builder
	lastWasSeparator := true
	for _, character := range strings.ToLower(strings.TrimSpace(providerName)) {
		isWordCharacter := (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9')
		if isWordCharacter {
			slug.WriteRune(character)
			lastWasSeparator = false
			continue
		}
		if !lastWasSeparator {
			slug.WriteRune('-')
			lastWasSeparator = true
		}
	}
	return strings.TrimSuffix(slug.String(), "-")
}

func medianFloat(values []float64) float64 {
	ordered := append([]float64{}, values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}
