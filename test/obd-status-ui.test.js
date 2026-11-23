'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
test('OBD status and route check use native ID7 menu items',async()=>{
    let app,draw;
    const text=[], handlers=new Map(), calls=[];
    const manifest=require('../apps/noahg_obdbridge/app.json');
    const screen=manifest.screens[0];
    assert.equal(screen.kind,'menu');
    assert(screen.items.some(([type,id,label])=>type==='button'&&id==='BTN_PROBE'&&label==='Check route'));
    assert(screen.items.some(([type,id])=>type==='scroll'&&id==='STATUS_TEXT'));
    const state={state:'listening_unverified',address:'172.16.222.97:6801',backend:'unverified',firewall:'scoped',tasu:'ready',
        version:'0.2.11',requests:0,responses:0,discoveryRequests:4,discoveryReplies:0,
        tasuVinError:'ecu_timeout',tasuVinTarget:'0x10',tasuTrace:{requests:2,target:'0x10',service:'0x22',verdict:'NO_ERROR',events:1,matched:0,ignored:1,lastEvent:'ECU 0x40 / SID 0x62 / 29 bytes / DID 0xF190',lastIgnore:'other_ecu'}};
    const main=fs.readFileSync(path.join(__dirname,'../apps/noahg_obdbridge/main.js'),'utf8');
    vm.runInNewContext(main,{require(name){
        if(name==='./kit/mgu')return{run(value){app=value;}};
        if(name==='./client')return{request:async(method,path)=>{calls.push([method,path]);return state;}};
        return require(name);
    },setTimeout(){return 1;},clearTimeout(){}});
    await app.start({ui:{onClick(id,fn){handlers.set(id,fn);},async text(id,value){assert.equal(id,'STATUS_TEXT');text.push(...value.split('\n'));}},app:{on(){}},loop(name,fps,fn){draw=fn;},theme(){return{bg:0,accent:1,text:2};},spec:{version:'0.2.11'}});
    await draw();
    assert.equal(text.length,9);
    await handlers.get('BTN_PROBE')();
    assert(calls.some(([method,path])=>method==='POST'&&path==='/probe'));
    assert(text.some(row=>row.includes('Verdict: NO_ERROR')&&row.includes('Events: 1')));
    assert(text.some(row=>row.includes('ECU 0x40')&&row.includes('other ecu')));
    assert(text.some(row=>row.includes('ECU reply timed out')));
});
