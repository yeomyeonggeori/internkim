package companyhost

import (
	"slices"
	"strings"
	"testing"
)

const deviceDumpList = `;
; Archive created at 2026-10-01 14:39:51 KST
;     dbname: blueclaw
;     Dumped from database version: 15.19
;
; Selected TOC Entries:
;
2; 3079 16394 EXTENSION - citext 
4101; 0 0 COMMENT - EXTENSION citext 
3; 3079 1130496 EXTENSION - pg_trgm 
4102; 0 0 COMMENT - EXTENSION pg_trgm 
4; 3079 1146880 EXTENSION - vector 
4103; 0 0 COMMENT - EXTENSION vector 
244; 1259 1187830 TABLE public memory_fact blueclaw
245; 1259 1187848 TABLE public memory_fact_embedding blueclaw
246; 1259 1187870 TABLE public memory_fact_circle blueclaw
247; 1259 1417216 TABLE public memory_fact_trigger blueclaw
248; 1259 1417232 TABLE public memory_fact_trigger_embedding blueclaw
4089; 0 1187830 TABLE DATA public memory_fact blueclaw
4090; 0 1187848 TABLE DATA public memory_fact_embedding blueclaw
4091; 0 1417216 TABLE DATA public memory_fact_trigger blueclaw
4092; 0 1417232 TABLE DATA public memory_fact_trigger_embedding blueclaw
3877; 2606 1187840 CONSTRAINT public memory_fact memory_fact_pkey blueclaw
3878; 2606 1187854 CONSTRAINT public memory_fact_embedding memory_fact_embedding_pkey blueclaw
3883; 2606 1417238 CONSTRAINT public memory_fact_trigger_embedding memory_fact_trigger_embedding_pkey blueclaw
3881; 2606 1417225 CONSTRAINT public memory_fact_trigger memory_fact_trigger_pkey blueclaw
3876; 1259 1187860 INDEX public memory_fact_embedding_hnsw_idx blueclaw
3916; 2606 1187855 FK CONSTRAINT public memory_fact_embedding memory_fact_embedding_fact_id_fkey blueclaw
3917; 2606 1187875 FK CONSTRAINT public memory_fact_circle memory_fact_circle_fact_id_fkey blueclaw
3918; 2606 1417239 FK CONSTRAINT public memory_fact_trigger_embedding memory_fact_trigger_embedding_trigger_id_fkey blueclaw
3919; 2606 1417226 FK CONSTRAINT public memory_fact_trigger memory_fact_trigger_fact_id_fkey blueclaw
`

func TestADeviceDumpRestoresWithoutTheVectorExtensionAndWhatIsTypedOnIt(t *testing.T) {
	kept, leftOut := withoutRetiredEntries(deviceDumpList)
	if !slices.Equal(slices.Sorted(slices.Values(leftOut)), slices.Sorted(slices.Values(retiredDumpEntries))) {
		t.Fatalf("left out %q, and the device's dump carries every retired entry once", leftOut)
	}
	for _, survivor := range []string{
		"EXTENSION - citext", "EXTENSION - pg_trgm",
		"TABLE public memory_fact blueclaw", "TABLE public memory_fact_circle blueclaw",
		"TABLE public memory_fact_trigger blueclaw", "TABLE DATA public memory_fact_trigger blueclaw",
		"CONSTRAINT public memory_fact_trigger memory_fact_trigger_pkey",
		"FK CONSTRAINT public memory_fact_trigger memory_fact_trigger_fact_id_fkey",
		"TABLE DATA public memory_fact blueclaw", "CONSTRAINT public memory_fact memory_fact_pkey",
		"FK CONSTRAINT public memory_fact_circle memory_fact_circle_fact_id_fkey",
		"; Selected TOC Entries:",
	} {
		if !strings.Contains(kept, survivor) {
			t.Errorf("the restore list lost %q, which needs no pgvector", survivor)
		}
	}
	if strings.Contains(kept, "vector") || strings.Contains(kept, "_embedding") {
		t.Errorf("the restore list still names a retired entry:\n%s", kept)
	}
}

func TestAHostsOwnDumpIsRestoredWhole(t *testing.T) {
	list := "; Selected TOC Entries:\n2; 3079 16394 EXTENSION - citext \n220; 1259 16400 TABLE public task internkim\n"
	kept, leftOut := withoutRetiredEntries(list)
	if len(leftOut) != 0 || kept != list {
		t.Fatalf("a dump with nothing retired lost %q", leftOut)
	}
}
