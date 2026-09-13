package ai

import (
	"testing"

	"ingestion/internal/gemini"
	"ingestion/internal/repository"
)

func TestPlanReembedding_SceneAdded(t *testing.T) {
	manifest := &gemini.GeminiMetadata{
		VideoTitle:     "Test",
		VideoCategory:  "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:       120,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 60, Name: "Scene 1", Content: "First", Items: []string{"a"}},
			{StartTime: 60, EndTime: 120, Name: "Scene 2", Content: "Second", Items: []string{"b"}},
		},
	}

	existing := []repository.VideoScene{
		{ID: "existing-1", StartTime: 0, EndTime: 60, Content: "First", Name: "Scene 1", ContentHash: hashContent(gemini.BuildDenseContent(manifest, &manifest.Scenes[0]))},
	}

	plan := planReembedding(manifest, existing)

	assertEqual(t, len(plan.ToCreate), 1, "should create 1 new scene")
	assertEqual(t, plan.ToCreate[0].Scene.StartTime, float64(60), "new scene should start at 60")
	assertEqual(t, len(plan.ToUpdate), 0, "should update nothing")
	assertEqual(t, plan.UnchangedCount, 1, "1 scene unchanged")
	assertEqual(t, len(plan.ToDeleteIDs), 0, "should delete nothing")
}

func TestPlanReembedding_SceneRemoved(t *testing.T) {
	manifest := &gemini.GeminiMetadata{
		VideoTitle:     "Test",
		VideoCategory:  "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:       60,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 60, Name: "Scene 1", Content: "First", Items: []string{"a"}},
		},
	}

	existing := []repository.VideoScene{
		{ID: "scene-1", StartTime: 0, EndTime: 60, Content: "First", Name: "Scene 1", ContentHash: hashContent(gemini.BuildDenseContent(manifest, &manifest.Scenes[0]))},
		{ID: "scene-2", StartTime: 60, EndTime: 120, Content: "Stale", Name: "Stale", ContentHash: "old"},
	}

	plan := planReembedding(manifest, existing)

	assertEqual(t, len(plan.ToDeleteIDs), 1, "should delete 1 stale scene")
	assertEqual(t, plan.ToDeleteIDs[0], "scene-2", "should delete the stale scene")
	assertEqual(t, plan.UnchangedCount, 1, "1 scene unchanged")
	assertEqual(t, len(plan.ToCreate), 0, "should create nothing")
	assertEqual(t, len(plan.ToUpdate), 0, "should update nothing")
}

func TestPlanReembedding_SceneContentChanged(t *testing.T) {
	manifest := &gemini.GeminiMetadata{
		VideoTitle:     "Test",
		VideoCategory:  "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:       60,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 60, Name: "Updated Scene", Content: "New description", Items: []string{"new"}},
		},
	}

	existing := []repository.VideoScene{
		{ID: "scene-1", StartTime: 0, EndTime: 60, Content: "Old description", Name: "Old Scene", ContentHash: "outdated-hash"},
	}

	plan := planReembedding(manifest, existing)

	assertEqual(t, len(plan.ToUpdate), 1, "should update 1 scene")
	assertEqual(t, plan.ToUpdate[0].ExistingID, "scene-1", "should update the existing scene")
	assertEqual(t, plan.ToUpdate[0].Scene.Name, "Updated Scene", "should carry new name")
	assertEqual(t, len(plan.ToCreate), 0, "should create nothing")
	assertEqual(t, plan.UnchangedCount, 0, "0 unchanged")
	assertEqual(t, len(plan.ToDeleteIDs), 0, "should delete nothing")
}

func TestPlanReembedding_SceneUnchanged(t *testing.T) {
	manifest := &gemini.GeminiMetadata{
		VideoTitle:     "Test",
		VideoCategory:  "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:       60,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 60, Name: "Scene 1", Content: "Same", Items: []string{"a"}},
		},
	}

	denseSoup := gemini.BuildDenseContent(manifest, &manifest.Scenes[0])

	existing := []repository.VideoScene{
		{ID: "scene-1", StartTime: 0, EndTime: 60, Content: "Same", Name: "Scene 1", ContentHash: hashContent(denseSoup)},
	}

	plan := planReembedding(manifest, existing)

	assertEqual(t, plan.UnchangedCount, 1, "1 scene unchanged")
	assertEqual(t, len(plan.ToCreate), 0, "should create nothing")
	assertEqual(t, len(plan.ToUpdate), 0, "should update nothing")
	assertEqual(t, len(plan.ToDeleteIDs), 0, "should delete nothing")
}

func TestPlanReembedding_AllAtOnce(t *testing.T) {
	manifest := &gemini.GeminiMetadata{
		VideoTitle:     "Test",
		VideoCategory:  "Tutorial",
		GlobalKeywords: []string{"test"},
		Duration:       180,
		Scenes: []gemini.SceneMetadata{
			{StartTime: 0, EndTime: 60, Name: "Unchanged", Content: "Same", Items: []string{"a"}},
			{StartTime: 60, EndTime: 120, Name: "Updated", Content: "New content", Items: []string{"b"}},
			{StartTime: 120, EndTime: 180, Name: "Brand new", Content: "Fresh", Items: []string{"c"}},
		},
	}

	unchangedSoup := gemini.BuildDenseContent(manifest, &manifest.Scenes[0])

	existing := []repository.VideoScene{
		{ID: "scene-unchanged", StartTime: 0, EndTime: 60, Content: "Same", Name: "Unchanged", ContentHash: hashContent(unchangedSoup)},
		{ID: "scene-update", StartTime: 60, EndTime: 120, Content: "Old content", Name: "Old", ContentHash: "stale"},
		{ID: "scene-delete", StartTime: 180, EndTime: 240, Content: "Gone", Name: "Deleted", ContentHash: "old"},
	}

	plan := planReembedding(manifest, existing)

	assertEqual(t, plan.UnchangedCount, 1, "1 unchanged")
	assertEqual(t, len(plan.ToUpdate), 1, "1 to update")
	assertEqual(t, plan.ToUpdate[0].ExistingID, "scene-update", "update correct scene")
	assertEqual(t, len(plan.ToCreate), 1, "1 to create")
	assertEqual(t, plan.ToCreate[0].Scene.StartTime, float64(120), "create correct scene")
	assertEqual(t, len(plan.ToDeleteIDs), 1, "1 to delete")
	assertEqual(t, plan.ToDeleteIDs[0], "scene-delete", "delete correct scene")
}
