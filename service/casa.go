package service

import (
	json2 "encoding/json"
	"regexp"
	"time"

	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/httper"
	"github.com/tidwall/gjson"
)

type CasaService interface {
	GetCasaosVersion() model.Version
}

type casaService struct{}

// forkReleasesAPI lists this fork's own GitHub releases (newest first). Used instead of
// upstream's api.casaos.io version-check endpoint, which reports IceWhaleTech's release
// schedule - completely unrelated to whether this fork itself has a newer build available.
const forkReleasesAPI = "https://api.github.com/repos/anonimo18032000/CasaOS/releases?per_page=1"

// forkReleaseVersionPattern pulls the leading dotted-numeric run out of a release tag like
// "v0.4.15-fork.1" (-> "0.4.15"), since IsNeedUpdate's comparison only understands plain
// numeric dot-segments and can't make sense of the "-fork.N" build suffix this fork's tags use.
// One consequence: two releases that only differ by that suffix (e.g. "-fork.1" vs "-fork.2")
// compare as equal, so bumping just the build number won't trigger the "update available" badge -
// only a change to the base version number will.
var forkReleaseVersionPattern = regexp.MustCompile(`^v?(\d+(?:\.\d+)*)`)

/**
 * @description: get remote version
 * @return {model.Version}
 */
func (o *casaService) GetCasaosVersion() model.Version {
	keyName := "casa_version"
	var version model.Version
	if result, ok := Cache.Get(keyName); ok {
		if dataStr, ok := result.(string); ok {
			_ = json2.Unmarshal([]byte(dataStr), &version)
			return version
		}
	}

	version = fetchLatestForkVersion()

	if len(version.Version) > 0 {
		if cached, err := json2.Marshal(version); err == nil {
			Cache.Set(keyName, string(cached), time.Minute*20)
		}
	}

	return version
}

// fetchLatestForkVersion queries this fork's GitHub releases directly rather than GitHub's
// "latest release" endpoint, because that endpoint ignores prereleases/drafts - and this fork's
// releases are all marked as prereleases (see the "-fork.N" tag suffix), so it would 404 forever.
func fetchLatestForkVersion() model.Version {
	var version model.Version

	head := map[string]string{
		"Accept":     "application/vnd.github+json",
		"User-Agent": "CasaOS",
	}
	body := httper.Get(forkReleasesAPI, head)

	releases := gjson.Parse(body).Array()
	if len(releases) == 0 {
		return version
	}

	release := releases[0]
	match := forkReleaseVersionPattern.FindStringSubmatch(release.Get("tag_name").String())
	if match == nil {
		return version
	}

	version.Version = match[1]
	version.ChangeLog = release.Get("body").String()
	return version
}

func NewCasaService() CasaService {
	return &casaService{}
}
