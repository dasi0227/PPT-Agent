package sqlite

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSnapshotRestoresEveryProjectTableAndIsolatesOtherProjects(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	statements := []string{
		`INSERT INTO projects(id,title,work_dir,created_at,updated_at) VALUES ('p','one','/p',1,1),('other','other','/other',1,1)`,
		`INSERT INTO threads(id,project_id,history_path,created_at,updated_at) VALUES ('t','p','threads/t.jsonl',1,1)`,
		`INSERT INTO slides(id,project_id,current_version) VALUES ('sl','p',3)`,
		`INSERT INTO runs(id,thread_id,project_id,scope_object,scope_slide_ids_json,scope_source_json,scope_revision,mode,run_command_json,status,created_at,updated_at) VALUES ('r','t','p','spec','[]','{}',1,'chat','{}','done',1,2)`,
		`INSERT INTO versions(id,target_type,target_id,version_no,snapshot_path,run_id,created_at) VALUES ('v','slide_html','project/p/slide-html-sl',1,'/p/versions/v','r',1),('d','design','p:design',1,'/p/versions/d','r',1),('o','design','other:design',1,'/other/d',NULL,1)`,
		`INSERT INTO run_events VALUES ('r',1,'run.started','{}',1)`,
		`INSERT INTO run_contexts VALUES ('r','ctx','default','hash',100,1000,'{}',1)`,
		`INSERT INTO steering_inbox(run_id,thread_id,client_message_id,request_hash,content,status,accepted_at) VALUES ('r','t','msg','hash','steering','injected',1)`,
		`INSERT INTO run_checkpoints VALUES ('rc','r','loop',1,'done','{}',1)`,
		`INSERT INTO context_index_snapshots VALUES ('idx','r','hash','{}',1)`,
		`INSERT INTO semantic_reviews VALUES ('review','r','finish',1,0.7,'hash','{}','{}',1)`,
		`INSERT INTO git_commit_operations(id,project_id,thread_id,client_request_id,model_profile,status,created_at,updated_at) VALUES ('git','p','t','req','model','completed',1,1)`,
		`INSERT INTO git_commit_events VALUES ('git',1,'completed','{}',1)`,
		`INSERT INTO briefing_versions(briefing_id,thread_id,project_id,kind,version_no,title,content,feedback,created_at) VALUES ('brief','t','p','handoff',1,'交接任务','summary','',1)`,
		`INSERT INTO context_compactions(id,thread_id,project_id,run_id,trigger,title,content,before_tokens,after_tokens,max_tokens,reclaimed_tokens,duration_ms,created_at) VALUES ('compact','t','p','r','auto','整理项目上下文','future summary',500,100,1000,400,1,1)`,
		`INSERT INTO command_activities(id,attempt_id,thread_id,project_id,kind,method,status,request,result,created_at,updated_at) VALUES ('cmd','attempt','t','p','polish','auto','completed','{}','{"title":"完善要求","content":"完整指令"}',1,1)`,
		`INSERT INTO idempotency_records VALUES ('create_run','t','req','hash','completed','{}',1,1)`,
		`INSERT INTO prompts VALUES ('global','global','global','','global library',1,1)`,
	}
	for _, sql := range statements {
		if err := s.db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	raw, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	var original map[string][]map[string]any
	if err = json.Unmarshal(raw, &original); err != nil {
		t.Fatal(err)
	}
	for _, table := range projectTables {
		if len(original[table]) == 0 {
			t.Fatalf("inventory did not capture %s", table)
		}
	}
	if err = s.db.Exec("DELETE FROM projects WHERE id = 'p'").Error; err != nil {
		t.Fatal(err)
	}
	if err = s.RestoreProject(ctx, "p", raw); err != nil {
		t.Fatal(err)
	}
	after, err := s.CaptureProject(ctx, "p")
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(raw) {
		t.Fatalf("snapshot round trip changed rows\nbefore %s\nafter %s", raw, after)
	}
	var n int64
	s.db.Raw("SELECT count(*) FROM projects WHERE id='other'").Scan(&n)
	if n != 1 {
		t.Fatal("other project removed")
	}
	s.db.Raw("SELECT count(*) FROM versions WHERE id='o'").Scan(&n)
	if n != 1 {
		t.Fatal("other versions removed")
	}
	s.db.Raw("SELECT count(*) FROM prompts WHERE id='global'").Scan(&n)
	if n != 1 {
		t.Fatal("global library removed")
	}
	// A bad DB payload rolls its entire replacement transaction back.
	original["runs"][0]["status"] = "invalid"
	broken, _ := json.Marshal(original)
	if err = s.RestoreProject(ctx, "p", broken); err == nil {
		t.Fatal("invalid snapshot committed")
	}
	after, _ = s.CaptureProject(ctx, "p")
	if string(after) != string(raw) {
		t.Fatal("failed transaction changed project")
	}
}
