#include <stdlib.h>

#include "helpers.h"
#include "_cgo_export.h"
#include "scip/message_default.h"
#include "scip/struct_message.h"

uintptr_t scipgo_branchruleId(SCIP_BRANCHRULE* branchrule) { return (uintptr_t)SCIPbranchruleGetData(branchrule); }
uintptr_t scipgo_eventhdlrId(SCIP_EVENTHDLR* eventhdlr) { return (uintptr_t)SCIPeventhdlrGetData(eventhdlr); }
uintptr_t scipgo_nodeselId(SCIP_NODESEL* nodesel) { return (uintptr_t)SCIPnodeselGetData(nodesel); }
uintptr_t scipgo_pricerId(SCIP_PRICER* pricer) { return (uintptr_t)SCIPpricerGetData(pricer); }
uintptr_t scipgo_heurId(SCIP_HEUR* heur) { return (uintptr_t)SCIPheurGetData(heur); }
uintptr_t scipgo_sepaId(SCIP_SEPA* sepa) { return (uintptr_t)SCIPsepaGetData(sepa); }
uintptr_t scipgo_conshdlrId(SCIP_CONSHDLR* conshdlr) { return (uintptr_t)SCIPconshdlrGetData(conshdlr); }

/* ------------------------------ branchrule ------------------------------ */

static SCIP_DECL_BRANCHFREE(scipgo_branchfree)
{
    return GoBranchFree(scip, branchrule);
}

static SCIP_DECL_BRANCHEXECLP(scipgo_branchexeclp)
{
    return GoBranchExecLp(scip, branchrule, allowaddcons, result);
}

static SCIP_DECL_BRANCHCOPY(scipgo_branchcopy)
{
    return GoBranchCopy(scip, branchrule);
}

SCIP_RETCODE scipgo_includeBranchrule(SCIP* scip, const char* name, const char* desc,
    int priority, int maxdepth, SCIP_Real maxbounddist, int hascopy, uintptr_t data)
{
    return SCIPincludeBranchrule(scip, name, desc, priority, maxdepth, maxbounddist,
        hascopy ? scipgo_branchcopy : NULL, scipgo_branchfree, NULL, NULL, NULL, NULL,
        scipgo_branchexeclp, NULL, NULL, (void*)data);
}

/* ------------------------------- eventhdlr ------------------------------ */

static SCIP_DECL_EVENTFREE(scipgo_eventfree)
{
    return GoEventhdlrFree(scip, eventhdlr);
}

static SCIP_DECL_EVENTINIT(scipgo_eventinit)
{
    return GoEventhdlrInit(scip, eventhdlr);
}

static SCIP_DECL_EVENTEXEC(scipgo_eventexec)
{
    return GoEventhdlrExec(scip, eventhdlr, event, eventdata);
}

static SCIP_DECL_EVENTCOPY(scipgo_eventcopy)
{
    return GoEventhdlrCopy(scip, eventhdlr);
}

SCIP_RETCODE scipgo_includeEventhdlr(SCIP* scip, const char* name, const char* desc,
    int hascopy, uintptr_t data)
{
    return SCIPincludeEventhdlr(scip, name, desc, hascopy ? scipgo_eventcopy : NULL,
        scipgo_eventfree, scipgo_eventinit, NULL, NULL, NULL, NULL, scipgo_eventexec, (void*)data);
}

/* -------------------------------- nodesel ------------------------------- */

static SCIP_DECL_NODESELFREE(scipgo_nodeselfree)
{
    return GoNodeselFree(scip, nodesel);
}

static SCIP_DECL_NODESELSELECT(scipgo_nodeselselect)
{
    return GoNodeselSelect(scip, nodesel, selnode);
}

static SCIP_DECL_NODESELCOMP(scipgo_nodeselcomp)
{
    return GoNodeselComp(scip, nodesel, node1, node2);
}

static SCIP_DECL_NODESELCOPY(scipgo_nodeselcopy)
{
    return GoNodeselCopy(scip, nodesel);
}

SCIP_RETCODE scipgo_includeNodesel(SCIP* scip, const char* name, const char* desc,
    int stdpriority, int memsavepriority, int hascopy, uintptr_t data)
{
    return SCIPincludeNodesel(scip, name, desc, stdpriority, memsavepriority,
        hascopy ? scipgo_nodeselcopy : NULL, scipgo_nodeselfree, NULL, NULL, NULL, NULL,
        scipgo_nodeselselect, scipgo_nodeselcomp, (void*)data);
}

/* --------------------------------- pricer ------------------------------- */

static SCIP_DECL_PRICERFREE(scipgo_pricerfree)
{
    return GoPricerFree(scip, pricer);
}

static SCIP_DECL_PRICERREDCOST(scipgo_pricerredcost)
{
    return GoPricerRedcost(scip, pricer, lowerbound, stopearly, result);
}

static SCIP_DECL_PRICERFARKAS(scipgo_pricerfarkas)
{
    return GoPricerFarkas(scip, pricer, result);
}

static SCIP_DECL_PRICERCOPY(scipgo_pricercopy)
{
    return GoPricerCopy(scip, pricer, valid);
}

SCIP_RETCODE scipgo_includePricer(SCIP* scip, const char* name, const char* desc,
    int priority, int delay, int hascopy, uintptr_t data)
{
    SCIP_RETCODE retcode = SCIPincludePricer(scip, name, desc, priority, delay,
        hascopy ? scipgo_pricercopy : NULL, scipgo_pricerfree, NULL, NULL, NULL, NULL,
        scipgo_pricerredcost, scipgo_pricerfarkas, (void*)data);
    if( retcode != SCIP_OKAY )
        return retcode;

    SCIP_PRICER* pricer = SCIPfindPricer(scip, name);
    return SCIPactivatePricer(scip, pricer);
}

/* -------------------------------- heuristic ----------------------------- */

static SCIP_DECL_HEURFREE(scipgo_heurfree)
{
    return GoHeurFree(scip, heur);
}

static SCIP_DECL_HEUREXEC(scipgo_heurexec)
{
    return GoHeurExec(scip, heur, heurtiming, nodeinfeasible, result);
}

static SCIP_DECL_HEURCOPY(scipgo_heurcopy)
{
    return GoHeurCopy(scip, heur);
}

SCIP_RETCODE scipgo_includeHeur(SCIP* scip, const char* name, const char* desc, char dispchar,
    int priority, int freq, int freqofs, int maxdepth, unsigned int timing, int usessubscip,
    int hascopy, uintptr_t data)
{
    return SCIPincludeHeur(scip, name, desc, dispchar, priority, freq, freqofs, maxdepth,
        (SCIP_HEURTIMING)timing, (SCIP_Bool)usessubscip, hascopy ? scipgo_heurcopy : NULL,
        scipgo_heurfree, NULL, NULL, NULL, NULL, scipgo_heurexec, (void*)data);
}

/* ------------------------------- separator ------------------------------ */

static SCIP_DECL_SEPAFREE(scipgo_sepafree)
{
    return GoSepaFree(scip, sepa);
}

static SCIP_DECL_SEPAEXECLP(scipgo_sepaexeclp)
{
    return GoSepaExecLp(scip, sepa, result, allowlocal, depth);
}

static SCIP_DECL_SEPAEXECSOL(scipgo_sepaexecsol)
{
    return GoSepaExecSol(scip, sepa, sol, result, allowlocal, depth);
}

static SCIP_DECL_SEPACOPY(scipgo_sepacopy)
{
    return GoSepaCopy(scip, sepa);
}

SCIP_RETCODE scipgo_includeSepa(SCIP* scip, const char* name, const char* desc, int priority,
    int freq, SCIP_Real maxbounddist, int usesubscip, int delay, int hascopy, uintptr_t data)
{
    return SCIPincludeSepa(scip, name, desc, priority, freq, maxbounddist,
        (SCIP_Bool)usesubscip, (SCIP_Bool)delay, hascopy ? scipgo_sepacopy : NULL,
        scipgo_sepafree, NULL, NULL, NULL, NULL, scipgo_sepaexeclp, scipgo_sepaexecsol, (void*)data);
}

/* ------------------------------- conshdlr ------------------------------- */

static SCIP_DECL_CONSFREE(scipgo_consfree)
{
    return GoConsFree(scip, conshdlr);
}

static SCIP_DECL_CONSENFOLP(scipgo_consenfolp)
{
    return GoConsEnfolp(scip, conshdlr, conss, nconss, nusefulconss, solinfeasible, result);
}

static SCIP_DECL_CONSCHECK(scipgo_conscheck)
{
    return GoConsCheck(scip, conshdlr, conss, nconss, sol, checkintegrality, checklprows,
        printreason, completely, result);
}

static SCIP_DECL_CONSLOCK(scipgo_conslock)
{
    return GoConsLock(scip, conshdlr, cons, locktype, nlockspos, nlocksneg);
}

static SCIP_DECL_CONSENFOPS(scipgo_consenfops)
{
    return GoConsEnfops(scip, conshdlr, conss, nconss, nusefulconss, solinfeasible,
        objinfeasible, result);
}

static SCIP_DECL_CONSSEPALP(scipgo_conssepalp)
{
    return GoConsSepalp(scip, conshdlr, conss, nconss, nusefulconss, result);
}

static SCIP_DECL_CONSPROP(scipgo_consprop)
{
    return GoConsProp(scip, conshdlr, conss, nconss, nusefulconss, nmarkedconss, proptiming,
        result);
}

static SCIP_DECL_CONSHDLRCOPY(scipgo_conshdlrcopy)
{
    return GoConsCopy(scip, conshdlr, valid);
}

SCIP_RETCODE scipgo_includeConshdlr(SCIP* scip, const char* name, const char* desc,
    int enfopriority, int checkpriority, int hascopy, int hasenfops,
    int hassepa, int sepafreq, int sepapriority, int delaysepa,
    int hasprop, int propfreq, int delayprop, unsigned int proptiming, uintptr_t data)
{
    SCIP_CONSHDLR* conshdlr = NULL;
    SCIP_CALL( SCIPincludeConshdlrBasic(scip, &conshdlr, name, desc,
        enfopriority, checkpriority, 0, FALSE,
        scipgo_consenfolp, hasenfops ? scipgo_consenfops : NULL,
        scipgo_conscheck, scipgo_conslock, (void*)data) );
    SCIP_CALL( SCIPsetConshdlrFree(scip, conshdlr, scipgo_consfree) );
    if( hascopy )
        SCIP_CALL( SCIPsetConshdlrCopy(scip, conshdlr, scipgo_conshdlrcopy, NULL) );
    if( hassepa )
        SCIP_CALL( SCIPsetConshdlrSepa(scip, conshdlr, scipgo_conssepalp, NULL,
            sepafreq, sepapriority, (SCIP_Bool)delaysepa) );
    if( hasprop )
        SCIP_CALL( SCIPsetConshdlrProp(scip, conshdlr, scipgo_consprop,
            propfreq, (SCIP_Bool)delayprop, (SCIP_PROPTIMING)proptiming) );
    return SCIP_OKAY;
}

/* ------------------------------ problem hook ---------------------------- */

static SCIP_DECL_PROBDELORIG(scipgo_probdelorig)
{
    (void)probdata;
    return GoProbDelorig(scip);
}

SCIP_RETCODE scipgo_watchProblem(SCIP* scip)
{
    return SCIPsetProbDelorig(scip, scipgo_probdelorig);
}

/* -------------------------------- copying ------------------------------- */

static SCIP_DECL_EVENTEXEC(scipgo_copyMarkerExec)
{
    (void)scip;
    (void)eventhdlr;
    (void)event;
    (void)eventdata;
    return SCIP_OKAY;
}

SCIP_RETCODE scipgo_includeCopyMarker(SCIP* scip, const char* name)
{
    if( SCIPfindEventhdlr(scip, name) != NULL )
        return SCIP_INVALIDDATA;

    /* No copy callback and no caught events: the marker belongs only to
       this instance and cannot be inherited by another SCIPcopy. It has no
       Go registry entry or data allocation requiring a free callback. */
    return SCIPincludeEventhdlrBasic(scip, NULL, name,
        "scipgo native copy identity", scipgo_copyMarkerExec, NULL);
}

SCIP_RETCODE scipgo_copyPlugins(SCIP* source, SCIP* target, SCIP_Bool* valid)
{
    return SCIPcopyPlugins(source, target,
        TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE,
        TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE, TRUE,
        TRUE, valid);
}

/* ---------------------------- message handler --------------------------- */

static void scipgo_messageLog(SCIP_MESSAGEHDLR* messagehdlr, FILE* file, const char* msg,
    void (*go)(uintptr_t, char*))
{
    /* SCIP normalizes a NULL file to stdout before the handler sees it, so
       "screen" output arrives as file == NULL || file == stdout (|| stderr).
       Any other FILE* means SCIP is writing to a specific file
       (SCIPprintStatistics and friends): write there and keep it out of the
       Go sink. */
    if( file != NULL && file != stdout && file != stderr )
    {
        fputs(msg, file);
        return;
    }
    go((uintptr_t)SCIPmessagehdlrGetData(messagehdlr), (char*)msg);
}

static SCIP_DECL_MESSAGEWARNING(scipgo_messagewarning)
{
    scipgo_messageLog(messagehdlr, file, msg, GoMessageWarning);
}

static SCIP_DECL_MESSAGEDIALOG(scipgo_messagedialog)
{
    scipgo_messageLog(messagehdlr, file, msg, GoMessageDialog);
}

static SCIP_DECL_MESSAGEINFO(scipgo_messageinfo)
{
    scipgo_messageLog(messagehdlr, file, msg, GoMessageInfo);
}

static SCIP_DECL_MESSAGEHDLRFREE(scipgo_messagehdlrfree)
{
    return GoMessageHdlrFree((uintptr_t)SCIPmessagehdlrGetData(messagehdlr));
}

SCIP_RETCODE scipgo_setMessagehdlr(SCIP* scip, uintptr_t data)
{
    SCIP_MESSAGEHDLR* hdlr;
    SCIP_CALL( SCIPmessagehdlrCreate(&hdlr, FALSE, NULL, FALSE,
        scipgo_messagewarning, scipgo_messagedialog, scipgo_messageinfo,
        scipgo_messagehdlrfree, (SCIP_MESSAGEHDLRDATA*)data) );
    SCIP_CALL( SCIPsetMessagehdlr(scip, hdlr) );
    SCIP_CALL( SCIPmessagehdlrRelease(&hdlr) ); /* SCIP captured its own reference */
    return SCIP_OKAY;
}

SCIP_RETCODE scipgo_setDefaultMessagehdlr(SCIP* scip)
{
    SCIP_MESSAGEHDLR* hdlr;
    SCIP_CALL( SCIPcreateMessagehdlrDefault(&hdlr, FALSE, NULL, FALSE) );
    SCIP_CALL( SCIPsetMessagehdlr(scip, hdlr) );
    SCIP_CALL( SCIPmessagehdlrRelease(&hdlr) );
    return SCIP_OKAY;
}

/* ----------------------------- error printing --------------------------- */

static SCIP_DECL_ERRORPRINTING(scipgo_errorprinting)
{
    /* same normalization as the message handler: NULL/stdout/stderr mean
       "screen" and are routed to Go, any other FILE* is a real target */
    if( file != NULL && file != stdout && file != stderr )
    {
        fputs(msg, file);
        return;
    }
    GoErrorPrinting((char*)msg);
}

void scipgo_setErrorPrinting(void)
{
    SCIPmessageSetErrorPrinting(scipgo_errorprinting, NULL);
}

void scipgo_setErrorPrintingDefault(void)
{
    SCIPmessageSetErrorPrintingDefault();
}


SCIP_RETCODE scipgo_enableExact(SCIP* scip, SCIP_Bool enable)
{
    return SCIPenableExactSolving(scip, enable);
}

/* Writes a malloc'd string representation (free with free()) of the rational
   produced by the caller; SCIPrationalStrLen sizes the buffer. */
static SCIP_RETCODE scipgo_rationalString(SCIP_RATIONAL* rat, char** str)
{
    int len;
    char* s;

    *str = NULL;
    len = SCIPrationalStrLen(rat);
    s = (char*)malloc((size_t)len + 1);
    if( s == NULL )
        return SCIP_NOMEMORY;
    SCIPrationalToString(rat, s, len + 1);
    *str = s;
    return SCIP_OKAY;
}

SCIP_RETCODE scipgo_solValExact(SCIP* scip, SCIP_SOL* sol, SCIP_VAR* var, char** str)
{
    SCIP_RATIONAL* rat = NULL;
    SCIP_RETCODE rc;

    *str = NULL;
    SCIP_CALL( SCIPrationalCreate(&rat) );
    SCIPgetSolValExact(scip, sol, var, rat); /* void: aborts on bad input */
    rc = scipgo_rationalString(rat, str);
    SCIPrationalFree(&rat);
    return rc;
}

SCIP_RETCODE scipgo_solOrigObjExact(SCIP* scip, SCIP_SOL* sol, char** str)
{
    SCIP_RATIONAL* rat = NULL;
    SCIP_RETCODE rc;

    *str = NULL;
    SCIP_CALL( SCIPrationalCreate(&rat) );
    SCIPgetSolOrigObjExact(scip, sol, rat); /* void: aborts on bad input */
    rc = scipgo_rationalString(rat, str);
    SCIPrationalFree(&rat);
    return rc;
}
