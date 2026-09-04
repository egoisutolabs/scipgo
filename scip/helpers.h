#ifndef SCIPGO_HELPERS_H
#define SCIPGO_HELPERS_H

#include <stdint.h>
#include <scip/scip.h>
#include <scip/scipdefplugins.h>
#include <scip/syncstore.h>
#include <tpi/tpi.h>

/* Go-side plugin registry ids travel as uintptr_t; only C ever holds them in
   the void* plugin-data slots, so the Go GC never sees a fake pointer. */
uintptr_t scipgo_branchruleId(SCIP_BRANCHRULE* branchrule);
uintptr_t scipgo_eventhdlrId(SCIP_EVENTHDLR* eventhdlr);
uintptr_t scipgo_nodeselId(SCIP_NODESEL* nodesel);
uintptr_t scipgo_pricerId(SCIP_PRICER* pricer);
uintptr_t scipgo_heurId(SCIP_HEUR* heur);
uintptr_t scipgo_sepaId(SCIP_SEPA* sepa);
uintptr_t scipgo_conshdlrId(SCIP_CONSHDLR* conshdlr);

/* hascopy: register a copy callback so the plugin is re-included into sub-SCIPs
   (LNS heuristics, concurrent solve workers). */
SCIP_RETCODE scipgo_includeBranchrule(SCIP* scip, const char* name, const char* desc,
    int priority, int maxdepth, SCIP_Real maxbounddist, int hascopy, uintptr_t data);

SCIP_RETCODE scipgo_includeEventhdlr(SCIP* scip, const char* name, const char* desc,
    int hascopy, uintptr_t data);

SCIP_RETCODE scipgo_includeNodesel(SCIP* scip, const char* name, const char* desc,
    int stdpriority, int memsavepriority, int hascopy, uintptr_t data);

SCIP_RETCODE scipgo_includePricer(SCIP* scip, const char* name, const char* desc,
    int priority, int delay, int hascopy, uintptr_t data);

SCIP_RETCODE scipgo_includeHeur(SCIP* scip, const char* name, const char* desc, char dispchar,
    int priority, int freq, int freqofs, int maxdepth, unsigned int timing, int usessubscip,
    int hascopy, uintptr_t data);

SCIP_RETCODE scipgo_includeSepa(SCIP* scip, const char* name, const char* desc, int priority,
    int freq, SCIP_Real maxbounddist, int usesubscip, int delay, int hascopy, uintptr_t data);

SCIP_RETCODE scipgo_includeConshdlr(SCIP* scip, const char* name, const char* desc,
    int enfopriority, int checkpriority, int hascopy, int hasenfops,
    int hassepa, int sepafreq, int sepapriority, int delaysepa,
    int hasprop, int propfreq, int delayprop, unsigned int proptiming, uintptr_t data);

/* Installs a delorig hook on the current original problem so Go learns when
   SCIP frees it (CreateProb, ReadProb, SCIPfree); PROBLEM stage only. */
SCIP_RETCODE scipgo_watchProblem(SCIP* scip);

/* Copies every plugin of source into target (all SCIPcopyPlugins flags on). */
SCIP_RETCODE scipgo_copyPlugins(SCIP* source, SCIP* target, SCIP_Bool* valid);

#endif /* SCIPGO_HELPERS_H */
