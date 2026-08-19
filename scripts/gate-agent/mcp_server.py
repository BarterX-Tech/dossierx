#!/usr/bin/env python3
"""mcp_server.py — the two report-only tools a gate reading agent may call.

WHY THIS EXISTS. gate/method.yaml grants exactly SurfaceFinding and
SurfaceVerdict, and neither is a tool any general-purpose harness ships. The
grant is only real if something PROVIDES those two tools and nothing else — a
harness that pre-approves two names while leaving Read/Bash/WebFetch in the
session's tool universe is the v0.5.2 defect documented in docs/RELEASING.md:
an agent held inside its bundle by nothing but a prompt instruction, which one
agent walked past with `grep` and then reported against itself. This server is
the half of the fix that makes the two granted names exist; the other half
(scripts/gate-agent/run.sh) makes them the ONLY names that exist.

BOTH TOOLS ARE REPORT-ONLY, AND THAT IS THE WHOLE DESIGN. They carry the
agent's answer OUT and can read NOTHING back in: no argument is a path, no
result echoes a byte the caller did not already have, and the only filesystem
write is the payload file the harness told this process to produce. If a tool
here ever grew a way to return repository bytes, the bundle would stop being
the agent's entire evidence set and every cache key in the gate — each one a
digest over that bundle — would silently stop being total.

WHAT IT SPEAKS. MCP over stdio: one JSON-RPC message per line, requests
answered in order, notifications absorbed. Deliberately minimal — initialize,
ping, tools/list, tools/call — because every capability beyond those four is a
capability an agent could reach that the grant never named.

CONFIGURATION IS BY ENVIRONMENT, SET BY THE RUNNER, NEVER BY THE AGENT:

  DOSSIERX_GATE_TOOLS       comma-separated tool names to advertise. The
                            runner passes the grant it was given, so the
                            session's tool universe IS the grant — an unknown
                            name here is a startup refusal, not a skip,
                            because a server that silently advertised less
                            than the grant would strand the agent with no way
                            to deliver a verdict, and one that advertised more
                            would widen the grant with no diff to review.
  DOSSIERX_GATE_ANSWER_OUT  where the payload lands when SurfaceVerdict is
                            called. Written atomically (temp + rename) so a
                            crash mid-write cannot leave a file that parses
                            far enough to look like an answer.

WHAT LANDS ON DISK is exactly the payload cmd/dossierx/gate_answer_test.go's
recorder accepts and nothing more:

  {"verdict": "PASS"|"FAILED", "findings": [...], "subjects": {...}}

The run identifier, the surface name and the fingerprint are the recorder's to
supply; a payload stating any of them is refused by the recorder, so this
server never writes them.

VALIDATION HERE IS A COURTESY, NOT THE AUTHORITY. The recorder
(TestGateAnswerRecord) re-validates everything with the collector's own
function and is the only judge that counts. This server checks the cheap
shapes — closed key sets, closed vocabularies, one verdict per session — so a
malformed call is refused in front of the agent, while it can still correct
itself, rather than after it has finished and been paid for. The vocabularies
restated here (consequence, blocking) are pinned on the Go side; if they
drift, the recorder refuses the payload and this file is the one to fix.

WHAT THIS FILE CANNOT PROMISE. It controls what these two tools do; it does
not control what other tools exist in the session. That containment is the
runner's (--tools "" plus --strict-mcp-config), and the runner proves it per
session rather than assuming it. And neither file can see what the model
provider does with the bundle on its side of the API — that boundary is
gate/method.yaml's "WHAT THIS FILE CANNOT PROMISE", restated here because it
is as true of the tools as of the grant.
"""

import json
import os
import sys
import tempfile

# ---------------------------------------------------------------------------
# the closed vocabularies, restated from the Go side
#
# gateConsequences and gateBlockingJudgements live in cmd/dossierx; the frame
# (gate/prompts/_frame.md) teaches the same three-and-two to every agent. They
# are restated rather than parsed out of the Go source because this server
# runs OUTSIDE the checkout, from an empty working directory, precisely so the
# agent's process has nothing of the repository within reach — a server that
# read the repo to learn its vocabulary would be the leak it exists to close.
# ---------------------------------------------------------------------------
CONSEQUENCES = ("acts-wrongly", "misled", "cosmetic")
BLOCKING = ("blocks", "deferrable")
VERDICTS = ("PASS", "FAILED")

FINDING_REQUIRED = ("surface", "rule", "consequence", "failure_scenario", "blocking", "detail")
FINDING_OPTIONAL = ("about",)
FINDING_KEYS = FINDING_REQUIRED + FINDING_OPTIONAL

# The full definitions of every tool this server COULD provide. What it DOES
# provide is the intersection with DOSSIERX_GATE_TOOLS — the grant decides,
# and a name the grant carries that this table lacks is a startup refusal.
TOOL_DEFINITIONS = {
    "SurfaceFinding": {
        "name": "SurfaceFinding",
        "description": (
            "Report one demonstrated mismatch on your surface. Call once per "
            "finding, before your single SurfaceVerdict call. Report-only: "
            "it records what you say and returns nothing you did not send."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "surface": {"type": "string", "description": "the surface this finding is raised for — the one you were assigned"},
                "rule": {"type": "string", "description": "a short kebab-case name for what is broken"},
                "about": {"type": "string", "description": "optional repository-relative path the defect really lives in, when it is not a document of your own surface"},
                "consequence": {"type": "string", "enum": list(CONSEQUENCES), "description": "what happens to a reader who believes the text"},
                "failure_scenario": {"type": "string", "description": "who is doing what and what goes wrong for them — a scenario a human can refute, never an adjective"},
                "blocking": {"type": "string", "enum": list(BLOCKING), "description": "your own judgement: does this stop the release"},
                "detail": {"type": "string", "description": "what the document says and what is actually true"},
            },
            "required": list(FINDING_REQUIRED),
            "additionalProperties": False,
        },
    },
    "SurfaceVerdict": {
        "name": "SurfaceVerdict",
        "description": (
            "Report your surface's verdict, exactly once, after all your "
            "SurfaceFinding calls. PASS or FAILED, plus the subjects map the "
            "frame requires — one entry for every declared subject."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "verdict": {"type": "string", "enum": list(VERDICTS)},
                "subjects": {
                    "type": "object",
                    "description": "one string value per subject the frame declares; 'not-claimed' for a subject your documents are silent on",
                    "additionalProperties": {"type": "string"},
                },
            },
            "required": ["verdict", "subjects"],
            "additionalProperties": False,
        },
    },
}


def die(reason):
    # A startup problem is a refusal on stderr and a dead server — never a
    # server that comes up offering some other tool set. The runner treats a
    # server that failed to connect as a failed run.
    sys.stderr.write("gate-agent mcp server: %s\n" % reason)
    sys.exit(2)


def granted_tools():
    raw = os.environ.get("DOSSIERX_GATE_TOOLS", "")
    names = [n.strip() for n in raw.split(",") if n.strip()]
    if not names:
        die("DOSSIERX_GATE_TOOLS is empty or unset; a server advertising no tools strands the agent with no way to report even FAILED, and the run reads as an agent that never answered")
    for name in names:
        if name not in TOOL_DEFINITIONS:
            die("DOSSIERX_GATE_TOOLS names %r, which this server does not implement. The grant is an exact set; a name it carries that nothing provides must refuse here, loudly, not surface later as a tool the agent silently never had" % name)
    return names


def answer_out():
    path = os.environ.get("DOSSIERX_GATE_ANSWER_OUT", "")
    if not path:
        die("DOSSIERX_GATE_ANSWER_OUT is unset; a verdict this server cannot write down is an agent paid for and not recorded")
    return path


class State(object):
    """One session's answer, assembled call by call.

    The payload is written ONLY when SurfaceVerdict lands. Findings before a
    verdict live in memory: a session that dies without a verdict leaves NO
    payload file, and the runner reads that absence as the failure it is — an
    agent that never finished is never mistaken for an agent that passed over
    zero findings.
    """

    def __init__(self):
        self.findings = []
        self.verdict_recorded = False


def tool_error(text):
    # An MCP tool error, not a JSON-RPC error: the agent sees it as the tool's
    # reply and can correct itself, which is the point of validating here at
    # all. isError=True is what marks it as a refusal rather than an answer.
    return {"content": [{"type": "text", "text": text}], "isError": True}


def tool_ok(text):
    return {"content": [{"type": "text", "text": text}], "isError": False}


def handle_finding(state, args):
    if not isinstance(args, dict):
        return tool_error("SurfaceFinding takes an object of fields, not %r." % type(args).__name__)
    extra = sorted(k for k in args if k not in FINDING_KEYS)
    if extra:
        hint = ""
        if "severity" in extra:
            hint = (" `severity` is not part of this schema any more: what replaced it is "
                    "`consequence` (one of %s), a `failure_scenario` stating who is doing what "
                    "and what goes wrong for them, and your own `blocking` judgement (%s)."
                    % (", ".join(CONSEQUENCES), " or ".join(BLOCKING)))
        return tool_error(
            "This finding states %s, which is not part of a finding. A finding carries exactly "
            "`%s` (and optionally `about`). A key dropped in silence would file a finding stating "
            "something other than what you reported, so it is refused instead.%s"
            % (", ".join("`%s`" % k for k in extra), "`, `".join(FINDING_REQUIRED), hint))
    missing = sorted(k for k in FINDING_REQUIRED if not isinstance(args.get(k), str) or not args.get(k).strip())
    if missing:
        return tool_error(
            "This finding is missing (or leaves blank) %s. Every one of `%s` is required — a "
            "finding a human cannot act on is not a finding."
            % (", ".join("`%s`" % k for k in missing), "`, `".join(FINDING_REQUIRED)))
    if args["consequence"] not in CONSEQUENCES:
        return tool_error("`consequence` is %r, which is not one of %s." % (args["consequence"], ", ".join(CONSEQUENCES)))
    if args["blocking"] not in BLOCKING:
        return tool_error("`blocking` is %r, which is not one of %s." % (args["blocking"], " or ".join(BLOCKING)))
    if "about" in args and (not isinstance(args["about"], str) or not args["about"].strip()):
        return tool_error("`about` was given but is not a non-empty string; omit it or name the repository-relative path the defect lives in.")

    # Recorded in call order, verbatim. Nothing is deduplicated, filtered or
    # re-graded on the way to the payload — every finding reaches the human,
    # and the human's ruling is the classification.
    recorded = {k: args[k] for k in FINDING_KEYS if k in args}
    state.findings.append(recorded)
    return tool_ok("Finding %d recorded for surface %r. Call SurfaceFinding again for further findings, then SurfaceVerdict exactly once."
                   % (len(state.findings), args["surface"]))


def handle_verdict(state, args, out_path):
    if not isinstance(args, dict):
        return tool_error("SurfaceVerdict takes an object of fields, not %r." % type(args).__name__)
    if state.verdict_recorded:
        # One verdict per session is the recorder's one-answer-per-surface
        # rule, enforced at the moment the second opinion is offered. Last-
        # wins over {FAILED, PASS} silently converts a FAILED into a PASS, so
        # the first answer stands and the second is refused with its reason.
        return tool_error("A verdict has already been recorded for this session. One agent gives one verdict; a second one would replace the first with nothing to say the first was ever given. This call was NOT recorded.")
    extra = sorted(k for k in args if k not in ("verdict", "subjects"))
    if extra:
        return tool_error(
            "SurfaceVerdict carries exactly `verdict` and `subjects`; this call also states %s. "
            "The run identifier, surface name and fingerprint are the harness's to supply, and "
            "findings travel by SurfaceFinding." % ", ".join("`%s`" % k for k in extra))
    if args.get("verdict") not in VERDICTS:
        return tool_error("`verdict` is %r; there are exactly two verdicts, PASS and FAILED. If you could not finish reading, the verdict is FAILED with a finding naming what you could not read." % args.get("verdict"))
    subjects = args.get("subjects")
    if not isinstance(subjects, dict) or not all(isinstance(k, str) and isinstance(v, str) for k, v in subjects.items()):
        return tool_error("`subjects` must be an object mapping every declared subject id to a string value ('not-claimed' for a subject your documents are silent on).")

    payload = {"verdict": args["verdict"], "findings": state.findings, "subjects": subjects}

    # Atomic: temp file in the destination directory, then rename. A reader —
    # the runner, racing us at session end — sees the whole payload or no
    # payload, never a truncated document that parses far enough to look like
    # an answer.
    out_dir = os.path.dirname(os.path.abspath(out_path)) or "."
    try:
        os.makedirs(out_dir, exist_ok=True)
        fd, tmp = tempfile.mkstemp(prefix=".payload-", suffix=".json", dir=out_dir)
        with os.fdopen(fd, "w") as f:
            json.dump(payload, f, indent=2)
            f.write("\n")
        os.chmod(tmp, 0o644)
        os.replace(tmp, out_path)
    except OSError as exc:
        # The agent is told, because from its side an unwritable verdict is a
        # verdict that was never given: it must not end its turn believing it
        # answered.
        return tool_error("The verdict could not be written down (%s). Your answer is NOT recorded; this run will read as an agent that never answered." % exc)

    state.verdict_recorded = True
    return tool_ok("Verdict %s recorded with %d finding(s). Your answer is complete; do not call SurfaceVerdict again." % (args["verdict"], len(state.findings)))


def main():
    tools = granted_tools()
    out_path = answer_out()
    state = State()

    stdout = sys.stdout

    def reply(msg):
        stdout.write(json.dumps(msg, separators=(",", ":")) + "\n")
        stdout.flush()

    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            msg = json.loads(line)
        except ValueError:
            # A parse error with no id has nothing to attach a response to;
            # JSON-RPC says answer id:null.
            reply({"jsonrpc": "2.0", "id": None, "error": {"code": -32700, "message": "parse error"}})
            continue

        method = msg.get("method", "")
        msg_id = msg.get("id")

        # Notifications carry no id and get no response — including
        # notifications/initialized and notifications/cancelled.
        if msg_id is None:
            continue

        if method == "initialize":
            client_version = (msg.get("params") or {}).get("protocolVersion", "2025-06-18")
            reply({"jsonrpc": "2.0", "id": msg_id, "result": {
                "protocolVersion": client_version,
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "dossierx-gate", "version": "1.0.0"},
            }})
        elif method == "ping":
            reply({"jsonrpc": "2.0", "id": msg_id, "result": {}})
        elif method == "tools/list":
            reply({"jsonrpc": "2.0", "id": msg_id, "result": {
                "tools": [TOOL_DEFINITIONS[name] for name in tools],
            }})
        elif method == "tools/call":
            params = msg.get("params") or {}
            name = params.get("name", "")
            args = params.get("arguments") or {}
            if name not in tools:
                # A name outside the grant is refused whether or not this
                # server could implement it: the grant is an exact set.
                reply({"jsonrpc": "2.0", "id": msg_id, "error": {"code": -32602, "message": "unknown tool %r; this session grants exactly: %s" % (name, ", ".join(tools))}})
                continue
            if name == "SurfaceFinding":
                result = handle_finding(state, args)
            else:
                result = handle_verdict(state, args, out_path)
            reply({"jsonrpc": "2.0", "id": msg_id, "result": result})
        else:
            # Everything else — resources, prompts, sampling, roots — is
            # deliberately not implemented. Each would be a capability beyond
            # the grant.
            reply({"jsonrpc": "2.0", "id": msg_id, "error": {"code": -32601, "message": "method %r not supported; this server provides tools only" % method}})


if __name__ == "__main__":
    main()
