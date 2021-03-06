package config

import (
	"github.com/zieckey/goini"
	"path/filepath"
	"os"
	"log"
	"fmt"
	"strings"
)

type Configuration interface {
	Section(section Section) map[string]string
	SectionGet(section Section, key Key, specifier string) (value string, ok bool)
	SectionGetOverrides(section Section, key Key) map[string]string
	NamedSections(section Section) []string
	NamedSection(name string, section Section) map[string]string
	NamedSectionGet(name string, section Section, key Key, specifier string) (value string, ok bool)
	NamedSectionGetOverrides(name string, section Section, key Key) map[string]string
	ApplyChanges(applyFunction func(mutator Mutator))
}

type configuration struct {
	ini     *goini.INI
	homeDir string
}

type Mutator interface {
	Delete(section Section)
	SectionSet(section Section, key Key, specifier, value string)
	SectionDelete(section Section, key Key, specifier string)
	NamedDelete(name string, section Section)
	NamedSectionSet(name string, section Section, key Key, specifier, value string)
	NamedSectionDelete(name string, section Section, key Key, specifier string)
}

type Section interface {
	Named() bool
	Name() string
}

type Key interface {
	Overloadable() bool
	Exportable() bool
	Name() string
}

type section struct {
	string
	bool
}

func (s section) Named() bool {
	return s.bool
}

func (s section) Name() string {
	return s.string
}

type key struct {
	string
	overloadable bool
	exportable   bool
}

func (k key) Overloadable() bool {
	return k.overloadable
}

func (k key) Exportable() bool {
	return k.exportable
}

func (k key) Name() string {
	return k.string
}

var (
	Remote Section = section{"Remote \"%s\"", true}
)

var sectionLookup = map[string]Section{
	"Remote": Remote,
}

var (
	Username          Key = key{"username", false, false}
	Password          Key = key{"password", false, false}
	Salt              Key = key{"salt", false, false}
	RemoteUser        Key = key{"user", false, true}
	ShowPrivate       Key = key{"show-private", false, true}
	RepositoryPattern Key = key{"repository-pattern", false, true}

	ReleasePattern        Key = key{"release-pattern", true, true}
	MilestonePattern      Key = key{"milestone-pattern", true, true}
	RepositoryBlacklisted Key = key{"repository-blacklisted", true, true}
	DownloadUrl           Key = key{"download-url", true, true}
)

var keyLookup = map[string]Key{
	Username.Name():              Username,
	Password.Name():              Password,
	Salt.Name():                  Salt,
	RemoteUser.Name():            RemoteUser,
	ShowPrivate.Name():           ShowPrivate,
	RepositoryPattern.Name():     RepositoryPattern,
	ReleasePattern.Name():        ReleasePattern,
	MilestonePattern.Name():      MilestonePattern,
	RepositoryBlacklisted.Name(): RepositoryBlacklisted,
	DownloadUrl.Name():           DownloadUrl,
}

func NewConfiguration(homeDir string) Configuration {
	grmPath := filepath.Join(homeDir, "github-release-monitor")
	configPath := filepath.Join(grmPath, "config")

	configuration := &configuration{homeDir: homeDir}

	if _, err := os.Stat(configPath); err != nil {
		return configuration
	}

	configuration.ini = goini.New()
	if err := configuration.ini.ParseFile(configPath); err != nil {
		log.Fatal(fmt.Sprintf("Could not read config file from '%s'", configPath), err)
	}

	return configuration
}

func KeyLookup(key string) Key {
	tokens := strings.Split(key, ":")
	return keyLookup[tokens[0]]
}

func SectionLookup(section string) Section {
	if !strings.Contains(section, " ") {
		return sectionLookup[section]
	}
	tokens := strings.Split(section, " \"")
	return sectionLookup[tokens[0]]
}

func ExtractSpecifier(key string) string {
	tokens := strings.Split(key, ":")
	if len(tokens) > 1 {
		return tokens[1]
	}
	tokens = strings.Split(key, "\"")
	if len(tokens) > 2 {
		return tokens[1]
	}
	return ""
}

func (c *configuration) Section(section Section) map[string]string {
	if section.Named() {
		log.Fatal("Tried to retrieve a named section without a name")
	}

	sectionName := section.Name()

	if kvmap, ok := c.ini.GetKvmap(sectionName); ok {
		overrides := make(map[string]string, len(kvmap))
		for k, v := range kvmap {
			overrides[k] = v
		}
		return overrides
	}
	return make(map[string]string, 0)
}

func (c *configuration) SectionGet(section Section, key Key, specifier string) (value string, ok bool) {
	if section.Named() {
		log.Fatal("Tried to retrieve a named section without a name")
	}
	sectionName := section.Name()
	return c.sectionGet(sectionName, key, specifier)
}

func (c *configuration) SectionGetOverrides(section Section, key Key) map[string]string {
	if section.Named() {
		log.Fatal("Tried to retrieve a named section without a name")
	}

	sectionName := section.Name()
	keySpace := fmt.Sprintf("%s:", key.Name())

	if kvmap, ok := c.ini.GetKvmap(sectionName); ok {
		overrides := make(map[string]string, 0)
		for k, v := range kvmap {
			if strings.HasPrefix(k, keySpace) {
				overrides[k] = v
			}
		}
		return overrides
	}
	return make(map[string]string, 0)
}

func (c *configuration) NamedSections(section Section) []string {
	sections := make([]string, 0)
	for iniSection := range c.ini.GetAll() {
		realSection := SectionLookup(iniSection)
		if realSection == section {
			sections = append(sections, iniSection)
		}
	}
	return sections
}

func (c *configuration) NamedSection(name string, section Section) map[string]string {
	if !section.Named() {
		log.Fatal("Tried to retrieve a non-named section with a name")
	}

	sectionName := buildSectionName(section, name)

	if kvmap, ok := c.ini.GetKvmap(sectionName); ok {
		overrides := make(map[string]string, len(kvmap))
		for k, v := range kvmap {
			overrides[k] = v
		}
		return overrides
	}
	return make(map[string]string, 0)
}

func (c *configuration) NamedSectionGet(name string, section Section, key Key, specifier string) (value string, ok bool) {
	if !section.Named() {
		log.Fatal("Tried to retrieve a non-named section with a name")
	}
	sectionName := buildSectionName(section, name)
	return c.sectionGet(sectionName, key, specifier)
}

func (c *configuration) NamedSectionGetOverrides(name string, section Section, key Key) map[string]string {
	if !section.Named() {
		log.Fatal("Tried to retrieve a non-named section with a name")
	}

	sectionName := buildSectionName(section, name)
	keySpace := fmt.Sprintf("%s:", key.Name())

	if kvmap, ok := c.ini.GetKvmap(sectionName); ok {
		overrides := make(map[string]string, 0)
		for k, v := range kvmap {
			if strings.HasPrefix(k, keySpace) {
				overrides[k] = v
			}
		}
		return overrides
	}
	return make(map[string]string, 0)
}

func (c *configuration) sectionGet(section string, key Key, specifier string) (value string, ok bool) {
	if key.Overloadable() && specifier != "" {
		if v, ok := c.ini.SectionGet(section, buildOverloadedKey(key, specifier)); ok {
			return v, true
		}
	}
	return c.ini.SectionGet(section, key.Name())
}

func (c *configuration) Delete(section Section) {
	if section.Named() {
		log.Fatal("Tried to delete a named section without a name")
	}
	sectionName := section.Name()
	sectionMap := c.ini.GetAll()
	delete(sectionMap, sectionName)
}

func (c *configuration) SectionSet(section Section, key Key, specifier, value string) {
	if section.Named() {
		log.Fatal("Tried to retrieve a named section without a name")
	}
	sectionName := section.Name()
	keySpace := key.Name()
	if specifier != "" {
		keySpace = buildOverloadedKey(key, specifier)
	}
	c.ini.SectionSet(sectionName, keySpace, value)
}

func (c *configuration) SectionDelete(section Section, key Key, specifier string) {
	if section.Named() {
		log.Fatal("Tried to retrieve a named section without a name")
	}
	sectionName := section.Name()
	keySpace := key.Name()
	if specifier != "" {
		keySpace = buildOverloadedKey(key, specifier)
	}
	c.ini.Delete(sectionName, keySpace)
}

func (c *configuration) NamedDelete(name string, section Section) {
	if !section.Named() {
		log.Fatal("Tried to delete a non-named section with a name")
	}
	sectionName := buildSectionName(section, name)
	sectionMap := c.ini.GetAll()
	delete(sectionMap, sectionName)
}

func (c *configuration) NamedSectionSet(name string, section Section, key Key, specifier, value string) {
	if !section.Named() {
		log.Fatal("Tried to retrieve a non-named section with a name")
	}
	sectionName := buildSectionName(section, name)
	keySpace := key.Name()
	if specifier != "" {
		keySpace = buildOverloadedKey(key, specifier)
	}
	c.ini.SectionSet(sectionName, keySpace, value)
}

func (c *configuration) NamedSectionDelete(name string, section Section, key Key, specifier string) {
	if !section.Named() {
		log.Fatal("Tried to retrieve a non-named section with a name")
	}
	sectionName := buildSectionName(section, name)
	keySpace := key.Name()
	if specifier != "" {
		keySpace = buildOverloadedKey(key, specifier)
	}
	c.ini.Delete(sectionName, keySpace)
}

func (c *configuration) ApplyChanges(applyFunction func(config Mutator)) {
	if c.ini == nil {
		c.ini = goini.New()
	}
	applyFunction(c)
	c.store()
}

func (c *configuration) store() {
	if c.ini == nil {
		return
	}

	grmPath := filepath.Join(c.homeDir, "github-release-monitor")
	configPath := filepath.Join(grmPath, "config")

	if _, err := os.Stat(grmPath); err != nil {
		if err := os.MkdirAll(grmPath, os.ModePerm); err != nil {
			log.Fatal(fmt.Sprintf("Could not create config directory '%s'", grmPath), err)
		}
	}

	file, err := os.Create(configPath)
	if err != nil {
		log.Fatal(fmt.Sprintf("Could not create config file '%s'", configPath), err)
	}

	c.ini.Write(file)
	println("Configuration written")
}

func buildSectionName(section Section, name string) string {
	return fmt.Sprintf(section.Name(), name)
}

func buildOverloadedKey(key Key, specifier string) string {
	return fmt.Sprintf("%s:%s", key.Name(), specifier)
}
