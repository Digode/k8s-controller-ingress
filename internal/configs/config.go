package configs

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	IngressClassNameSuffix string
	LabelSubdomain         string
	LabelPath              string
	Tls                    bool
	Annotation             Annotation
	Domain                 Domain
}

type Annotation struct {
	Key      string
	Privates []string
	Publics  []string
}

type Domain struct {
	Privates []string
	Publics  []string
}

const (
	keyIngressClassNameSuffix = "ingressClassNameSuffix"
	keyLabelSubdomain         = "labelSubdomain"
	keyLabelPath              = "labelPath"
	keyTls                    = "tls"
	keyAnnotationKey          = "annotation.key"
	keyAnnotationPrivates     = "annotation.privates"
	keyAnnotationPublics      = "annotation.publics"
	keyDomainPrivates         = "domain.privates"
	keyDomainPublics          = "domain.publics"
)

var config *Config = nil

func Get() *Config {
	if config == nil {
		config = load()
		log.Printf("Annotation Key loaded: %s", config.Annotation.Key)
		log.Printf("LabelSubdomain loaded: %s", config.LabelSubdomain)
		log.Printf("LabelPath loaded: %s", config.LabelPath)
		log.Printf("IngressClassNameSuffix loaded: %s", config.IngressClassNameSuffix)
		log.Printf("Annotation.Privates loaded: %+v", config.Annotation.Privates)
		log.Printf("Annotation.Publics loaded: %+v", config.Annotation.Publics)
		log.Printf("Domain loaded: %s", config.Domain)
		log.Printf("Tls loaded: %v", config.Tls)
	}
	return config
}

func load() *Config {
	ingressClassNameSuffix := getEnvString(keyIngressClassNameSuffix)
	labelSubdomain := getEnvString(keyLabelSubdomain)
	labelPath := getEnvString(keyLabelPath)
	tls := getEnvBoolean(keyTls)
	annotationKey := getEnvString(keyAnnotationKey)
	annotationPrivates := getEnvStringSlice(keyAnnotationPrivates)
	annotationPublics := getEnvStringSlice(keyAnnotationPublics)
	domainPrivates := getEnvStringSlice(keyDomainPrivates)
	domainPublics := getEnvStringSlice(keyDomainPublics)

	return &Config{
		IngressClassNameSuffix: ingressClassNameSuffix,
		LabelSubdomain:         labelSubdomain,
		LabelPath:              labelPath,
		Tls:                    tls,
		Annotation: Annotation{
			Key:      annotationKey,
			Privates: annotationPrivates,
			Publics:  annotationPublics,
		},
		Domain: Domain{
			Privates: domainPrivates,
			Publics:  domainPublics,
		},
	}
}

func getEnvStringSlice(key string) []string {
	value := os.Getenv(key)
	if len(value) < 1 {
		return []string{}
	}
	return strings.Split(value, ",")
}

func getEnvString(key string) string {
	value := os.Getenv(key)
	if len(value) < 1 {
		return ""
	}
	return value
}

func getEnvBoolean(key string) bool {
	value := os.Getenv(key)
	if len(value) < 1 {
		return false
	}
	val, err := strconv.ParseBool(value)

	if err != nil {
		return false
	}

	return val
}