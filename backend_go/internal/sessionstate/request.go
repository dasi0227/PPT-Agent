package sessionstate

import "context"

type requestSceneKey struct{}
type interactionRevisionKey struct{}

func WithRequestScene(ctx context.Context, scene int64) context.Context {
	return context.WithValue(ctx, requestSceneKey{}, scene)
}

func RequestScene(ctx context.Context) (int64, bool) {
	scene, ok := ctx.Value(requestSceneKey{}).(int64)
	return scene, ok && scene > 0
}

func WithInteractionRevision(ctx context.Context, revision int64) context.Context {
	return context.WithValue(ctx, interactionRevisionKey{}, revision)
}

func InteractionRevision(ctx context.Context) (int64, bool) {
	revision, ok := ctx.Value(interactionRevisionKey{}).(int64)
	return revision, ok && revision > 0
}

type entityRevisionKey struct{}

func WithEntityRevision(ctx context.Context, revision int64) context.Context {
	return context.WithValue(ctx, entityRevisionKey{}, revision)
}
func EntityRevision(ctx context.Context) (int64, bool) {
	revision, ok := ctx.Value(entityRevisionKey{}).(int64)
	return revision, ok && revision > 0
}
