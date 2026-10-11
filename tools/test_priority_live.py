#!/usr/bin/env python3
"""Submit one owned probe, held by default; --eligible allows its finite CPU task."""
import argparse, json, subprocess, uuid
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--host',required=True)
parser.add_argument('--submit',action='store_true',required=True)
parser.add_argument('--eligible',action='store_true',help='allow one finite CPU-only probe to run; otherwise hold it')
args=parser.parse_args()
host=args.host
report={'host':host,'scope':'one finite CPU-only synthetic probe' if args.eligible else 'one held CPU-only synthetic job; never released'}
job=None
token='perch-priority-'+uuid.uuid4().hex[:16]
def ssh(command):
    return subprocess.run(['ssh','-o','BatchMode=yes','-o','ConnectTimeout=10',host,command],capture_output=True,text=True,timeout=25,check=True).stdout.strip()
try:
    user=ssh('id -un')
    result=ssh("sbatch --parsable "+("" if args.eligible else "--hold ")+"--job-name=perch-priority-probe --comment="+token+" --nodes=1 --ntasks=1 --mem=32M --time=1 --output=/dev/null --error=/dev/null --wrap=true")
    assert result.isdecimal(),result
    job=result;report['job_id']=job
    row=ssh('squeue --jobs='+job+' --noheader --format="%i|%u|%j|%k|%T|%R"').split('|')
    assert row[:4]==[job,user,'perch-priority-probe',token],row
    if not args.eligible: assert row[4]=='PENDING' and 'Held' in row[5],row
    report['queue']=row
    p=subprocess.run([str(ROOT/'bin/perch'),'priority','--host',host,'--json'],capture_output=True,text=True,timeout=30,check=True)
    response=json.loads(p.stdout)
    report['priority']=response
    report['probe_records']=[r for r in response['data'] if r['allocation_id']==job]
    report['stage']='observed' if report['probe_records'] else 'no-factors-reported'
finally:
    if job:
        row=ssh('squeue --jobs='+job+' --noheader --format="%i|%u|%j|%k"')
        if row:
            assert row.split('|')==[job,user,'perch-priority-probe',token]
            ssh('scancel --full '+job)
            assert not ssh('squeue --jobs='+job+' --noheader --format="%i"')
            report['queue_absence_verified']=True
    (ROOT/('output/validation/priority-eligible-live.json' if args.eligible else 'output/validation/priority-held-live.json')).write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(report,indent=2))
