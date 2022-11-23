/*
 * Copyright © 2022 Docker, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package query

import (
	"sync"

	"github.com/docker/index-cli-plugin/types"
)

type queryResult struct {
	Vulnerabilities []types.VulnerabilitiesByPurl
	BaseImages      []types.BaseImageMatch
	Image           *types.BaseImage
	Error           error
}

func ForCvesAndBaseImagesAsync(sb *types.Sbom, includeCves bool, includeBaseImages bool, workspace string, apiKey string) *types.Sbom {
	resultChan := make(chan queryResult, 3)
	var wg sync.WaitGroup
	if includeCves {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cves, err := ForVulnerabilitiesInGraphQL(sb)
			if err != nil {
				resultChan <- queryResult{
					Error: err,
				}
			}
			if cves != nil {
				resultChan <- queryResult{
					Vulnerabilities: cves.VulnerabilitiesByPackage,
				}
			}
		}()
	}
	if includeBaseImages {
		wg.Add(2)
		go func() {
			defer wg.Done()
			bi, err := ForBaseImageInGraphQL(sb.Source.Image.Config, true)
			if err != nil {
				resultChan <- queryResult{
					Error: err,
				}
			}
			if bi != nil && len(bi.ImagesByDiffIds) > 0 {
				resultChan <- queryResult{
					BaseImages: bi.ImagesByDiffIds,
				}
			}
		}()
		go func() {
			defer wg.Done()
			bi, err := ForImageInGraphQL(sb)
			if err != nil {
				resultChan <- queryResult{
					Error: err,
				}
			}
			if bi != nil && bi.ImageDetailsByDigest.Digest != "" {
				resultChan <- queryResult{
					Image: &bi.ImageDetailsByDigest,
				}
			}
		}()
	}
	wg.Wait()
	close(resultChan)

	for result := range resultChan {
		if result.BaseImages != nil {
			sb.Source.BaseImages = result.BaseImages
		}
		if result.Vulnerabilities != nil {
			sb.Vulnerabilities = result.Vulnerabilities
		}
		if result.Image != nil {
			sb.Source.Image.Details = result.Image
		}
	}

	return sb
}
