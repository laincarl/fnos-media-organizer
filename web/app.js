const $ = id => document.getElementById(id);
let missingPermissionDirs=[],permissionGuideKey="",lastExecution='',taskResultsFilter='running',taskFilterChosen=false;
let state={}, selected=new Set(), pollTimer, authenticated=false, lastItems='',lastInspection='',lastPlan='',lastResults='',previewInputs='',settingsLoaded=false,dirtySettings=false,previewOpen=false,statusFilter='all',selectionAnchor=null;
const statusNames={ready:'待整理',blocked:'已跳过',unchanged:'已符合规范',unchecked:'待检查',done:'完成',failed:'失败',running:'整理中',waiting:'等待整理',stopped:'未执行'};
const defaultRules={movie_directory:'{category}/{title} ({year})',movie_filename:'{title} ({year})',episode_directory:'{category}/{title} ({year})/Season {season}',episode_filename:'{title} - S{season}E{episode} - {episode_title}'};
const ruleFields={movie_directory:'movieDirectory',movie_filename:'movieFilename',episode_directory:'episodeDirectory',episode_filename:'episodeFilename'};
function el(tag,text,className){const n=document.createElement(tag);if(text!==undefined)n.textContent=text;if(className)n.className=className;return n;}
function message(text){const target=authenticated?$('message'):$('loginFeedback');$('message').hidden=true;$('loginFeedback').hidden=true;if(!authenticated&&text==='请使用 NAS 账号授权连接影视')text='';target.textContent=text;target.hidden=!text;}
let activePage='library',pageScroll={};
function showTab(name){const content=document.querySelector('main');pageScroll[activePage]=content.scrollTop;activePage=name;setLibraryOpen(false);document.querySelectorAll('.page').forEach(p=>p.hidden=p.id!=='page-'+name);content.scrollTop=pageScroll[name]||0;$('selectionActions').hidden=name==='settings';$('progress').hidden=name==='settings';$('saveSettings').hidden=name!=='settings';$('settingsSaved').hidden=name!=='settings';$('openSettings').hidden=!authenticated||name==='settings';renderTaskProgress();(name==='settings'?$('backLibrary'):$('openSettings')).focus();}
$('openSettings').addEventListener('click',()=>showTab('settings'));
$('backLibrary').addEventListener('click',()=>showTab('library'));
async function api(path,data){const res=await fetch('/app/media-organizer/api/'+path,{method:data===undefined?'GET':'POST',headers:data===undefined?{}:{'Content-Type':'application/json','X-Organizer-Request':'1'},body:data===undefined?undefined:JSON.stringify(data)});const out=await res.json();if(!res.ok){if(res.status===401&&path!=='login'){authenticated=false;document.body.classList.add('login-view');renderTaskProgress();$('workspace').hidden=true;$('openSettings').hidden=true;$('login').hidden=false;closePreview();}throw Object.assign(Error(out.error||'请求失败'),{permissionIssues:out.permission_issues});}return out;}
let libraryOptions=[],libraryActive=0;
function renderLibraries(libs){libraryOptions=libs.map(x=>({value:x.guid,label:x.title||x.name||x.guid}));if(!libraryOptions.some(x=>x.value===$('library').value))$('library').value=libraryOptions[0]?.value||'';const buttons=libraryOptions.map((o,i)=>{const b=el('button',undefined,'select-option');b.type='button';b.id='library-option-'+i;b.setAttribute('role','option');b.tabIndex=-1;b.append(el('span','', 'option-check'),el('span',o.label));b.addEventListener('click',()=>chooseLibrary(i));return b;});$('libraryOptions').replaceChildren(...buttons);syncLibraryPicker();}
function syncLibraryPicker(){const i=libraryOptions.findIndex(x=>x.value===$('library').value);$('libraryValue').textContent=libraryOptions[i]?.label||'请先连接影视';[...$('libraryOptions').children].forEach((b,n)=>{b.setAttribute('aria-selected',n===i?'true':'false');b.classList.toggle('selected',n===i);b.classList.toggle('highlighted',n===libraryActive);});}
function setLibraryOpen(open){$('libraryOptions').hidden=!open;$('libraryTrigger').setAttribute('aria-expanded',String(open));if(open){libraryActive=Math.max(0,libraryOptions.findIndex(x=>x.value===$('library').value));syncLibraryPicker();$('libraryTrigger').setAttribute('aria-activedescendant','library-option-'+libraryActive);}else{$('libraryTrigger').removeAttribute('aria-activedescendant');}}
function chooseLibrary(i){if(state.busy||!libraryOptions[i])return;const changed=$('library').value!==libraryOptions[i].value;$('library').value=libraryOptions[i].value;syncLibraryPicker();setLibraryOpen(false);$('libraryTrigger').focus();if(changed)$('library').dispatchEvent(new Event('change'));}
$('libraryTrigger').addEventListener('click',()=>setLibraryOpen($('libraryOptions').hidden));
$('libraryTrigger').addEventListener('keydown',e=>{if(['ArrowDown','ArrowUp','Home','End'].includes(e.key)){e.preventDefault();if($('libraryOptions').hidden)setLibraryOpen(true);else{libraryActive=e.key==='Home'?0:e.key==='End'?libraryOptions.length-1:(libraryActive+(e.key==='ArrowDown'?1:-1)+libraryOptions.length)%libraryOptions.length;syncLibraryPicker();$('libraryTrigger').setAttribute('aria-activedescendant','library-option-'+libraryActive);}}else if(e.key==='Enter'||e.key===' '){e.preventDefault();$('libraryOptions').hidden?setLibraryOpen(true):chooseLibrary(libraryActive);}else if(e.key==='Escape'){e.preventDefault();setLibraryOpen(false);}else if(e.key==='Tab')setLibraryOpen(false);});
document.addEventListener('click',e=>{if(!$('libraryPicker').contains(e.target))setLibraryOpen(false);});
function settings(){return {rules:Object.fromEntries(Object.entries(ruleFields).map(([key,id])=>[key,$(id).value.trim()]))};}
function currentInputs(){return JSON.stringify({...settings(),library:$('library').value,ids:[...selected].sort()});}
function ruleExample(){const r=settings().rules,values={title:'示例影视',year:'2024',category:'电影',season:'01',episode:'02',episode_title:'重逢'},expand=x=>x.replace(/\{(\w+)\}/g,(token,k)=>values[k]??token);$('ruleExample').replaceChildren(...[['电影',expand(r.movie_directory)+'/'+expand(r.movie_filename)+'.mkv'],['剧集',expand(r.episode_directory.replace('{category}','电视剧'))+'/'+expand(r.episode_filename)+'.mkv']].map(([label,path])=>{const row=el('div',undefined,'example-row');row.append(el('span',label,'example-label'),el('code',path));return row;}));}
function loadSettings(s){for(const [key,id]of Object.entries(ruleFields))$(id).value=s.rules?.[key]??defaultRules[key];settingsLoaded=true;dirtySettings=false;$('settingsSaved').textContent='命名规则已加载';ruleExample();}
function pathDiff(source,target){const box=el('div',undefined,'path-diff');if(!target||source===target){box.append(el('div',source,'diff-same'));return box;}const a=Array.from(source),b=Array.from(target);let first=0,last=0;while(first<a.length&&first<b.length&&a[first]===b[first])first++;while(last<a.length-first&&last<b.length-first&&a[a.length-1-last]===b[b.length-1-last])last++;for(const [chars,tag,label]of[[a,'del','−'],[b,'ins','+']]){const line=el('div',undefined,'diff-line');line.append(el('span',label,'diff-label'),document.createTextNode(chars.slice(0,first).join('')),el(tag,chars.slice(first,chars.length-last).join('')||'∅'),document.createTextNode(last?chars.slice(-last).join(''):''));box.append(line);}return box;}
function table(headers){const t=el('table'),h=el('thead'),r=el('tr');headers.forEach(x=>r.append(el('th',x)));h.append(r);t.append(h,el('tbody'));return t;}
function checkbox(value,checked,change,eventType='change'){const c=el('input');c.type='checkbox';c.value=value;c.checked=checked;c.addEventListener(eventType,change);return c;}
function inspectionMap(){const map=new Map();if(!dirtySettings)for(const r of state.inspection?.rows||[])map.set(r.id+'|'+r.source,r);return map;}
function fileState(m){
 if(state.task?.active&&state.task.file_ids?.includes(m.id)&&!(state.results||[]).some(r=>r.id===m.id)){
  if(state.task.active&&state.task.current_id===m.id)return {status:'running',reason:`第 ${Math.min(state.task.completed+1,state.task.total)} / ${state.task.total} 个`};
  return {status:state.task.active?'waiting':'stopped',reason:state.task.active?'':'整理已停止，文件保持原路径'};
 }
 return inspectionMap().get(m.id+'|'+m.source);
}
function filterFileStatus(status){return ({done:'unchanged',failed:'blocked',running:'ready',waiting:'ready',stopped:'ready'})[status]||status;}
function visibleItems(){const map=inspectionMap(),query=$('search').value.trim().toLowerCase(),filter=statusFilter;return (state.items||[]).filter(m=>{const row=fileState(m),status=filterFileStatus(row?.status||'unchecked');return (!query||(m.title+' '+m.source).toLowerCase().includes(query))&&(filter==='all'||filter===status);});}
function selectFile(id,event){
 if(state.busy)return;
 const items=visibleItems(),index=items.findIndex(m=>m.id===id),anchor=items.findIndex(m=>m.id===selectionAnchor);
 if(index<0)return;
 const range=event.shiftKey&&anchor>=0?items.slice(Math.min(anchor,index),Math.max(anchor,index)+1):[items[index]];
 for(const m of range)event.target.checked?selected.add(m.id):selected.delete(m.id);
 if(!event.shiftKey||anchor<0)selectionAnchor=id;
 invalidatePlan();renderSelection();
}
function renderFiles(){const map=inspectionMap(),items=visibleItems(),t=table(['选择','封面','影视 / 季集','当前文件路径','目录规范']);t.className='media-table';const selectVisible=checkbox('all',false,e=>{if(state.busy)return;selectionAnchor=null;for(const m of visibleItems()){e.target.checked?selected.add(m.id):selected.delete(m.id);}invalidatePlan();renderSelection();});selectVisible.id='selectVisible';selectVisible.setAttribute('aria-label','全选当前结果');const selectionLabel=el('label',undefined,'table-select-all');selectionLabel.append(selectVisible,el('span','全选当前结果','selection-label'));t.querySelector('th').replaceChildren(selectionLabel);for(const m of items){const r=fileState(m),status=r?.status||'unchecked',tr=el('tr'),td=el('td');const rowCheckbox=checkbox(m.id,selected.has(m.id),e=>selectFile(m.id,e),'click');rowCheckbox.title='按住 Shift 点击可连续选择或取消选择';rowCheckbox.setAttribute('aria-label','选择 '+m.title);td.append(rowCheckbox);const name=el('td');name.append(el('strong',m.title+(m.year?' ('+m.year+')':'')));if(m.kind==='episode')name.append(el('span',`S${String(m.season).padStart(2,'0')}E${String(m.episode).padStart(2,'0')}${m.episode_title?' · '+m.episode_title:''}`));const paths=el('td',undefined,'path');paths.append(status==='ready'&&r.target?pathDiff(m.source,r.target):el('div',m.source));const result=el('td');result.append(el('span',statusNames[status],'status '+status));if(r?.reason&&status!=='unchanged')result.append(el('div',r.reason,'hint'));const cover=el('td'),frame=el('div','暂无封面','poster');if(m.poster){const img=el('img');img.alt=m.title+'封面';img.loading='lazy';img.src='/app/media-organizer/api/poster?id='+encodeURIComponent(m.id);img.addEventListener('error',()=>img.remove(),{once:true});frame.append(img);}cover.append(frame);tr.append(td,cover,name,paths,result);t.lastChild.append(tr);}$('files').replaceChildren(items.length?t:el('p',state.items?.length?'当前筛选没有结果。':'暂无文件，请选择影视库并读取识别结果。','empty'));$('fileCount').textContent=`文件列表 · ${items.length} / ${(state.items||[]).length}`;const checked=[...map.values()];$('statTotal').textContent=(state.items||[]).length;for(const [id,status]of[['statOk','unchanged'],['statPending','ready'],['statBlocked','blocked']])$(id).textContent=state.inspection&&!dirtySettings?checked.filter(x=>x.status===status).length:'—';renderSelection();}
function renderSelection(){const visible=visibleItems(),count=visible.filter(m=>selected.has(m.id)).length,all=$('selectVisible');if(all){all.checked=visible.length>0&&count===visible.length;all.indeterminate=count>0&&count<visible.length;all.disabled=!!state.busy||!visible.length;document.querySelectorAll('#files tbody input[type=checkbox]').forEach(c=>{c.checked=selected.has(c.value);c.disabled=!!state.busy;});}$('selectionCount').textContent=`已选 ${selected.size} 个`;$('preview').disabled=!selected.size||!!state.busy;}
function renderRows(target,rows,selectable){const t=table(selectable?['执行','影视','原路径 → 目标路径','状态']:['影视','原路径 → 目标路径','结果']);t.className=selectable?'plan-table':'result-table';rows.forEach((r,i)=>{const tr=el('tr');if(selectable){const td=el('td'),c=checkbox(String(i),r.status==='ready',updateExecute);c.disabled=r.status!=='ready';td.append(c);tr.append(td);}const name=el('td');name.append(el('strong',r.title));const paths=el('td',undefined,'path');paths.append(pathDiff(r.source,r.target));const result=el('td');result.append(el('span',statusNames[r.status]||r.status,'status '+r.status));if(r.reason&&r.status!=='unchanged')result.append(el('div',r.reason,'hint'));tr.append(name,paths,result);t.lastChild.append(tr);});$(target).replaceChildren(t);if(selectable)updateExecute();}
function updateExecute(){$('execute').disabled=!!state.busy||!$('plan').querySelector('input:checked:not(:disabled)');}
function invalidatePlan(){previewInputs='';lastPlan='';$('previewCard').hidden=true;closePreview();}
function requireSettings(){if(dirtySettings){showTab('settings');throw Error('请先保存命名规则。');}if(!state.directories?.length)throw Error('请先读取影视库。');}
function setTaskPopover(open,restoreFocus=false){
 $('taskPopover').hidden=!open;$('openTaskProgress').setAttribute('aria-expanded',String(open));
 if(open)$('closeTaskProgress').focus();else if(restoreFocus)$('openTaskProgress').focus();
}
$('openTaskProgress').addEventListener('click',()=>setTaskPopover($('taskPopover').hidden));
$('closeTaskProgress').addEventListener('click',()=>setTaskPopover(false,true));
document.addEventListener('click',e=>{if(!$('taskControl').contains(e.target))setTaskPopover(false);});
document.addEventListener('keydown',e=>{if(e.key==='Escape'&&!$('taskPopover').hidden){e.preventDefault();setTaskPopover(false,true);}});
function renderTaskProgress(){
 const task=state.task,hasTask=!!task?.total,active=hasTask?!!task.active:!!state.busy;
 $('taskControl').hidden=!authenticated;if(!authenticated)setTaskPopover(false);
 $('openTaskProgress').classList.toggle('running',active);
 $('openTaskProgress').setAttribute('aria-label',active?'任务正在执行，查看进度':'查看任务进度');
 $('openTaskProgress').title=active?'任务正在执行':'任务进度';
 $('progress').hidden=activePage==='settings';$('selectionActions').hidden=activePage==='settings';
 $('preview').textContent=task?.active?'整理中…':'整理所选文件';
 if(!hasTask)return;
 const completed=Math.min(task.total,Math.max(0,task.completed));
 const summary=task.active?'正在整理':completed===task.total?(task.failed?'整理完成，部分失败':'整理完成'):'整理已停止';
 if(activePage==='library')$('progress').textContent=`${summary} · ${completed}/${task.total}`;
}
function renderPermissionGuide(){
 const key=JSON.stringify(missingPermissionDirs);if(key===permissionGuideKey)return;permissionGuideKey=key;
 $('permissionGuide').hidden=true;$('permissionCopyStatus').textContent='';
 if(!missingPermissionDirs.length){$('permissionSteps').open=false;return;}
 const total=missingPermissionDirs.reduce((n,d)=>n+d.files,0);
 $('permissionSummary').textContent=`${total} 个文件位于 ${missingPermissionDirs.length} 个未授权目录，授权后即可检查和整理。`;
 $('missingDirectories').replaceChildren(...missingPermissionDirs.map(d=>{
  const row=el('div',undefined,'missing-directory'),path=el('code',d.path),count=el('span',d.files+' 个文件','hint'),copy=el('button','复制目录');copy.type='button';
  copy.addEventListener('click',async()=>{try{if(!navigator.clipboard?.writeText)throw Error();await navigator.clipboard.writeText(d.path);$('permissionCopyStatus').textContent='目录已复制';}catch{const range=document.createRange();range.selectNodeContents(path);const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);$('permissionCopyStatus').textContent='目录已选中，请复制；手机可长按目录文字复制。';}});
  row.append(path,count,copy);return row;
 }));
}
$('showPermissionSteps').addEventListener('click',()=>{$('permissionSteps').open=!$('permissionSteps').open;});
$('recheckPermissions').addEventListener('click',()=>{if(!state.busy)$('scan').click();});
function render(){renderPermissionGuide();$('recheckPermissions').disabled=!!state.busy;document.body.classList.toggle('busy',!!state.busy);$('progress').textContent=state.progress||'就绪';if(state.error)message(state.error);for(const id of['scan','saveSettings'])$(id).disabled=!!state.busy||(id==='scan'&&!state.libraries?.length);renderSelection();$('version').textContent=state.version?.version?'影视 '+state.version.version+' / mediasrv '+state.version.mediasrvVersion:'';
 $('account').textContent='当前账号：'+(state.account||'影视管理员');$('directories').textContent=(state.directories||[]).join('\n')||'读取影视库后显示';
 const libs=state.libraries||[],libKey=JSON.stringify(libs.map(x=>[x.guid,x.title,x.name]));if($('library').dataset.key!==libKey){$('library').dataset.key=libKey;renderLibraries(libs);}$('libraryTrigger').disabled=!!state.busy||!libs.length;

 const itemsKey=JSON.stringify(state.items||[]),inspectionKey=JSON.stringify(state.inspection);if(itemsKey!==lastItems){lastItems=itemsKey;selected=new Set([...selected].filter(id=>(state.items||[]).some(m=>m.id===id)));selectionAnchor=null;renderFiles();}if(inspectionKey!==lastInspection){lastInspection=inspectionKey;renderFiles();}$('warnings').textContent=(state.warnings||[]).join('；');
 const executionKey=JSON.stringify([state.task,state.results,state.busy]);if(executionKey!==lastExecution){lastExecution=executionKey;renderFiles();renderTaskResults();}
 const visiblePlan=previewInputs===currentInputs()?state.plan:null,planKey=JSON.stringify(visiblePlan);if(planKey!==lastPlan){lastPlan=planKey;$('previewCard').hidden=!visiblePlan;if(visiblePlan){const rows=visiblePlan.rows||[];renderRows('plan',rows,true);$('planCount').textContent=`${rows.filter(x=>x.status==='ready').length} 个可移动 / ${rows.length} 个文件`;}}
 const resultKey=JSON.stringify(state.results||[]);if(resultKey!==lastResults){lastResults=resultKey;renderTaskResults();}$('previewModal').hidden=!previewOpen||!visiblePlan;document.body.classList.toggle('modal-open',previewOpen&&!!visiblePlan);updateExecute();renderTaskProgress();}
async function refresh(){if(!authenticated)return;try{const r=await api('state');state=r.state;missingPermissionDirs=r.missing_directories||[];$('roots').textContent=r.roots?.length?r.roots.join('\n'):'尚未获得目录授权';if(!settingsLoaded)loadSettings(r.settings||{});render();}catch(e){message(e.message);}finally{clearTimeout(pollTimer);if(authenticated)pollTimer=setTimeout(refresh,state.busy?800:4000);}}
async function action(fn){message('');try{await fn();await refresh();}catch(e){if(e.permissionIssues?.length)openPermissionDialog(e.permissionIssues);else message(e.message);}}
$('settingsForm').addEventListener('submit',e=>{e.preventDefault();action(async()=>{await api('settings',settings());dirtySettings=false;invalidatePlan();$('settingsSaved').textContent='规则已保存并重新检查';});});
$('scan').addEventListener('click',()=>action(()=>{if(dirtySettings)throw Error('请先保存命名规则。');invalidatePlan();return api('scan',{library:$('library').value});}));

$('preview').addEventListener('click',()=>action(async()=>{requireSettings();taskFilterChosen=false;const snapshot=currentInputs();await api('preview',{ids:[...selected]});previewInputs=snapshot;previewOpen=true;await refresh();$('cancelPreview').focus();}));
function closePreview(){previewOpen=false;$('previewModal').hidden=true;document.body.classList.remove('modal-open');$('preview').focus();}
$('closePreview').addEventListener('click',closePreview);
$('cancelPreview').addEventListener('click',closePreview);
$('execute').addEventListener('click',()=>action(async()=>{
 const indices=[...$('plan').querySelectorAll('input:checked:not(:disabled)')].map(x=>Number(x.value));
 if(!state.plan||!indices.length||previewInputs!==currentInputs())throw Error('整理预览已失效，请重新点击整理。');
 await api('execute',{plan_id:state.plan.id,indices});closePreview();showTab('library');
}));
document.addEventListener('keydown',e=>{
 if($('previewModal').hidden)return;
 if(e.key==='Escape'){e.preventDefault();closePreview();}
 if(e.key==='Tab'){const controls=[...$('previewModal').querySelectorAll('button:not(:disabled),input:not(:disabled)')];const first=controls[0],last=controls.at(-1);if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus();}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus();}}
});
function markDirty(){dirtySettings=true;$('settingsSaved').textContent='有未保存的规则';invalidatePlan();renderFiles();ruleExample();}
for(const id of Object.values(ruleFields))$(id).addEventListener('input',markDirty);
$('resetRules').addEventListener('click',()=>{for(const [key,id]of Object.entries(ruleFields))$(id).value=defaultRules[key];markDirty();});
$('library').addEventListener('change',()=>action(()=>{invalidatePlan();selected.clear();selectionAnchor=null;if(dirtySettings)throw Error('请先保存命名规则，再读取所选影视库。');return api('scan',{library:$('library').value});}));$('search').addEventListener('input',()=>{selectionAnchor=null;renderFiles();});
document.querySelectorAll('[data-status]').forEach(button=>button.addEventListener('click',()=>{selectionAnchor=null;statusFilter=button.dataset.status;document.querySelectorAll('[data-status]').forEach(b=>{b.classList.toggle('active',b===button);b.setAttribute('aria-pressed',b===button?'true':'false');});renderFiles();}));
$('mediaLogin').addEventListener('click',async e=>{e.preventDefault();const button=$('mediaLogin');button.disabled=true;button.classList.add('connecting');button.querySelector('span').textContent='正在前往授权…';message('');try{const r=await api('auth/start',{origin:location.origin});location.assign(r.url);}catch(e){message(e.message);button.disabled=false;button.classList.remove('connecting');button.querySelector('span').textContent='使用 NAS 账号连接';}});
async function autoConnect(){message('');try{const r=await api('state');authenticated=true;document.body.classList.remove('login-view');state=r.state;$('login').hidden=true;$('workspace').hidden=false;$('openSettings').hidden=!$('page-settings').hidden;await refresh();}catch(e){message(e.message);}}
$('retry').addEventListener('click',async()=>{const button=$('retry');button.disabled=true;button.textContent='检测中…';await autoConnect();button.disabled=false;button.textContent='重新检测';if(!authenticated&&!$('loginFeedback').textContent)message('尚未连接，请先使用 NAS 账号授权。');});
autoConnect();

function resultPermissionIssues(row){let path=row.source.slice(0,row.source.lastIndexOf('/'));const match=(state.directories||[]).filter(d=>row.source.startsWith(d.replace(/\/$/,'')+'/')).sort((a,b)=>b.length-a.length)[0];if(match)path=match;return [{path,files:1,reason:'移动时发现目录权限不足'}];}
function renderTaskResults(){
 const results=state.results||[],pending=state.task?.active?(state.items||[]).filter(m=>state.task.file_ids?.includes(m.id)&&!results.some(r=>r.id===m.id)).map(m=>({...m,status:state.task.current_id===m.id?'running':'waiting',reason:state.task.current_id===m.id?'正在移动和重命名':'等待整理'})):[];
 const groups={running:pending,done:results.filter(r=>r.status==='done'),failed:results.filter(r=>r.status==='failed')};
 const hasTask=!!state.task?.total;$('taskResultTabs').hidden=!hasTask;$('taskResults').hidden=false;if(!hasTask){$('taskResults').replaceChildren(el('p','暂无执行中的任务','task-result-empty'));return;}
 if(!taskFilterChosen)taskResultsFilter=state.task?.active?'running':groups.failed.length?'failed':'done';
 for(const tab of document.querySelectorAll('[data-task-filter]')){const active=tab.dataset.taskFilter===taskResultsFilter;tab.setAttribute('aria-selected',String(active));tab.tabIndex=active?0:-1;tab.querySelector('span').textContent=groups[tab.dataset.taskFilter].length;}
 $('taskResults').setAttribute('aria-labelledby',({running:'taskTabRunning',done:'taskTabDone',failed:'taskTabFailed'})[taskResultsFilter]);const rows=groups[taskResultsFilter];
 if(!rows.length){$('taskResults').replaceChildren(el('p',({running:'暂无进行中的文件',done:'暂无已完成的文件',failed:'暂无失败的文件'})[taskResultsFilter],'task-result-empty'));return;}
 $('taskResults').replaceChildren(...rows.map(r=>{const item=el('div',undefined,'task-result');const head=el('div',undefined,'task-result-head');head.append(el('span',r.title+(r.kind==='episode'?` · S${String(r.season).padStart(2,'0')}E${String(r.episode).padStart(2,'0')}`:'')),el('span',statusNames[r.status]||r.status,'status '+r.status));item.append(head,el('p',r.status==='done'?r.target:r.reason,'task-result-detail'));if(r.cleaned_directories?.length){const cleanup=el('details');cleanup.append(el('summary',`已清理 ${r.cleaned_directories.length} 个空目录`),...r.cleaned_directories.map(path=>el('p',path,'task-result-detail')));item.append(cleanup);}if(r.cleanup_warning)item.append(el('p',r.cleanup_warning,'task-result-detail'));if(r.status==='failed'){const source=el('details');source.append(el('summary','查看文件路径'),el('p',r.source,'task-result-detail'));item.append(source);}if(r.issue_code==='permission'){const button=el('button','添加目录权限','result-permission');button.type='button';button.addEventListener('click',()=>{setTaskPopover(false);openPermissionDialog(resultPermissionIssues(r));});item.append(button);}return item;}));}
let permissionReturnFocus=null;
function openPermissionDialog(issues){permissionReturnFocus=document.activeElement;if(previewOpen)closePreview();setTaskPopover(false);$('permissionModalPaths').replaceChildren(...issues.map(d=>{const box=el('div',undefined,'permission-modal-path'),path=el('code',d.path),copy=el('button','复制目录');copy.type='button';copy.addEventListener('click',async()=>{try{await navigator.clipboard.writeText(d.path);$('permissionModalCopy').textContent='目录已复制';}catch{const range=document.createRange();range.selectNodeContents(path);const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);$('permissionModalCopy').textContent='目录已选中，请复制；手机可长按目录文字复制。';}});box.append(el('p',d.reason,'hint'),path,copy);return box;}));$('permissionModalCopy').textContent='';$('permissionModal').hidden=false;document.body.classList.add('modal-open');$('closePermission').focus();}
function closePermissionDialog(){$('permissionModal').hidden=true;document.body.classList.remove('modal-open');if(permissionReturnFocus?.isConnected)permissionReturnFocus.focus();}
$('closePermission').addEventListener('click',closePermissionDialog);$('dismissPermission').addEventListener('click',closePermissionDialog);
document.addEventListener('keydown',e=>{if($('permissionModal').hidden)return;if(e.key==='Escape'){e.preventDefault();closePermissionDialog();}if(e.key==='Tab'){const controls=[...$('permissionModal').querySelectorAll('button,a[href]')],first=controls[0],last=controls.at(-1);if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus();}else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus();}}});

for(const tab of document.querySelectorAll('[data-task-filter]')){tab.addEventListener('click',()=>{taskFilterChosen=true;taskResultsFilter=tab.dataset.taskFilter;renderTaskResults();});tab.addEventListener('keydown',e=>{const keys=['ArrowLeft','ArrowRight','Home','End'];if(!keys.includes(e.key))return;e.preventDefault();const tabs=[...document.querySelectorAll('[data-task-filter]')],i=tabs.indexOf(tab),n=e.key==='Home'?0:e.key==='End'?tabs.length-1:(i+(e.key==='ArrowRight'?1:-1)+tabs.length)%tabs.length;tabs[n].click();tabs[n].focus();});}
