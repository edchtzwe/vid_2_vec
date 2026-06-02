package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
)

// CHANGE: Moved all diffing, hashing, and indexing logic from embedder.go to this new file.

type reembedPlan struct {
	ToCreate       []plannedScene
	ToUpdate[]plannedUpdate
	ToDeleteIDs[]string
	UnchangedCount int
}

type plannedScene struct {
	Scene     gemini.SceneMetadata
	DenseSoup string
	NewHash   string
}

type plannedUpdate struct {
	ExistingID string
	Scene      gemini.SceneMetadata
	DenseSoup  string
	NewHash    string
}

// planReembedding is a pure function that compares the manifest against the database
// to determine exactly what needs to be created, updated, or deleted.
func planReembedding(manifest *gemini.GeminiMetadata, existingScenes[]repository.VideoScene) reembedPlan {
	sceneIndex := buildSceneIndex(existingScenes)
	var plan reembedPlan
	var processedIDs[]string

	for _, scene := range manifest.Scenes {
		denseSoup := gemini.BuildDenseContent(manifest, &scene)
		newHash := hashContent(denseSoup)
		key := sceneKey(scene.StartTime, scene.EndTime)

		existing, found := sceneIndex[key]

		switch {
		case found && existing.ContentHash == newHash:
			plan.UnchangedCount++
			processedIDs = append(processedIDs, existing.ID)
		case found:
			plan.ToUpdate = append(plan.ToUpdate, plannedUpdate{
				ExistingID: existing.ID,
				Scene:      scene,
				DenseSoup:  denseSoup,
				NewHash:    newHash,
			})
			processedIDs = append(processedIDs, existing.ID)
		default:
			plan.ToCreate = append(plan.ToCreate, plannedScene{
				Scene:     scene,
				DenseSoup: denseSoup,
				NewHash:   newHash,
			})
		}
	}

	// Find IDs that exist in the DB but were not in the new manifest
	processedMap := make(map[string]bool, len(processedIDs))
	for _, id := range processedIDs {
		processedMap[id] = true
	}

	for _, existing := range existingScenes {
		if !processedMap[existing.ID] {
			plan.ToDeleteIDs = append(plan.ToDeleteIDs, existing.ID)
		}
	}

	return plan
}

func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// sceneKey produces a lookup key from a time range. Using "%.3f" avoids
// floating-point formatting mismatches while keeping millisecond precision.
func sceneKey(start, end float64) string {
	return fmt.Sprintf("%.3f:%.3f", start, end)
}

func buildSceneIndex(scenes []repository.VideoScene) map[string]*repository.VideoScene {
	idx := make(map[string]*repository.VideoScene, len(scenes))
	for i := range scenes {
		key := sceneKey(scenes[i].StartTime, scenes[i].EndTime)
		idx[key] = &scenes[i]
	}
	return idx
}