package run

import (
 "context"
 "errors"
 "testing"

 "github.com/dasi0227/PPT-Agent/backend/internal/model"
 "github.com/dasi0227/PPT-Agent/backend/internal/sessionstate"
 "github.com/dasi0227/PPT-Agent/backend/internal/workflow"
)

type capturingUpdateStore struct {
 *memStore
 updates []sessionstate.RunUpdate
 scene int64
 owner string
 execution int64
 err error
}
func (s *capturingUpdateStore) ApplyRunUpdate(ctx context.Context, _ string, update sessionstate.RunUpdate) (sessionstate.Receipt,error) {
 s.scene,_ = sessionstate.RequestScene(ctx)
 s.owner,s.execution,_ = workflow.CheckpointOwnership(ctx)
 s.updates=append(s.updates,update)
 return sessionstate.Receipt{},s.err
}

func TestBusSubmitsTypedFactsWithExecutionFence(t *testing.T) {
 store:=&capturingUpdateStore{memStore:newMemStore()}
 row:=model.Run{ID:"r",SceneRevision:7,OwnerInstanceID:"worker",ExecutionRevision:4}
 bus:=NewBus(row,store)
 payload:=model.RunProgressPayload{PublicEventBase:model.NewPublicEventBase(row.ID),Activity:model.ActivityRunAnalyzing}
 if err:=bus.Emit(context.Background(),model.EventRunProgress,payload);err!=nil {t.Fatal(err)}
 if len(store.updates)!=1 || store.updates[0].Type!=model.EventRunProgress || store.scene!=7 || store.owner!="worker" || store.execution!=4 {t.Fatalf("missing current-state fence: %+v",store)}
 failure:=errors.New("commit failed")
 store.err=failure
 if err:=bus.Emit(context.Background(),model.EventRunProgress,payload);!errors.Is(err,failure) {t.Fatalf("write failure lost: %v",err)}
}

func TestBusRejectsMalformedPublicPayloadBeforeStore(t *testing.T) {
 store:=&capturingUpdateStore{memStore:newMemStore()}
 bus:=NewBus(model.Run{ID:"r",SceneRevision:1},store)
 if err:=bus.Emit(context.Background(),model.EventToolCompleted,model.ToolCompletedPayload{});!errors.Is(err,ErrEventProtocol) {t.Fatalf("invalid payload: %v",err)}
 if len(store.updates)!=0 {t.Fatal("invalid public payload entered storage")}
}
