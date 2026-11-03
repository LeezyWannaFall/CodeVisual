import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import type { GraphNode } from '../api/types'
import { Icon } from './Icon'
export function RoutePicker({routes,value,onChange}: {routes:GraphNode[];value:GraphNode|null;onChange:(node:GraphNode)=>void}) {
 const [open,setOpen]=useState(false), [query,setQuery]=useState(''), [active,setActive]=useState(0)
 const [position,setPosition]=useState({top:0,left:0,width:360,maxHeight:360})
 const trigger=useRef<HTMLButtonElement>(null), popup=useRef<HTMLDivElement>(null), search=useRef<HTMLInputElement>(null), id=useId()
 const options=value&&!routes.some(n=>n.id===value.id)?[value,...routes]:routes
 const filtered=options.filter(n=>`${n.label} ${n.attributes.http?.scope ?? ''} ${n.attributes.http?.handler ?? ''}`.toLowerCase().includes(query.toLowerCase()))
 function close(){setOpen(false);trigger.current?.focus({preventScroll:true})}
 function choose(node:GraphNode){onChange(node);close()}
 useLayoutEffect(()=>{
  if(!open)return
  const place=()=>{const r=trigger.current!.getBoundingClientRect(),w=Math.min(440,window.innerWidth-24),below=window.innerHeight-r.bottom-12,above=r.top-12,up=below<220&&above>below,h=Math.max(100,Math.min(360,up?above:below));setPosition({left:Math.max(12,Math.min(r.left,window.innerWidth-w-12)),top:up?r.top-h-6:r.bottom+6,width:w,maxHeight:h})}
  place();search.current?.focus({preventScroll:true})
  window.addEventListener('resize',place);window.addEventListener('scroll',place,true)
  return()=>{window.removeEventListener('resize',place);window.removeEventListener('scroll',place,true)}
 },[open])
 useEffect(()=>{if(!open)return;const dismiss=(e:PointerEvent)=>{if(!popup.current?.contains(e.target as Node)&&!trigger.current?.contains(e.target as Node))setOpen(false)};document.addEventListener('pointerdown',dismiss);return()=>document.removeEventListener('pointerdown',dismiss)},[open])
 function content(node:GraphNode){return <><span className="picker-method">{node.attributes.http?.method ?? 'ƒ'}</span><span className="picker-label"><strong>{node.attributes.http?.path ?? node.label}</strong><small title={node.attributes.http?.scope}>{node.attributes.http?.scope.split('/').slice(-2).join('/') ?? node.attributes.relativePath}</small></span></>}
 return <><button ref={trigger} className="route-picker-trigger" role="combobox" aria-label="Flow entry point" aria-expanded={open} aria-controls={open?id:undefined} aria-haspopup="listbox" onClick={()=>{setOpen(v=>!v);setQuery('');setActive(0)}} onKeyDown={e=>{if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();if(!open){setQuery('');setActive(0)}setOpen(true)}}}>{value?content(value):<span>Choose a route</span>}<Icon name="arrow" size={12}/></button>
 {open&&createPortal(<div ref={popup} className="route-picker-popover" style={position} onBlur={e=>{if(!e.currentTarget.contains(e.relatedTarget as Node)&&e.relatedTarget!==trigger.current)setOpen(false)}} onKeyDown={e=>{if(e.key==='Escape'){e.preventDefault();close()}else if(['ArrowDown','ArrowUp','Home','End'].includes(e.key)){e.preventDefault();const next=e.key==='Home'?0:e.key==='End'?filtered.length-1:Math.max(0,Math.min(filtered.length-1,active+(e.key==='ArrowDown'?1:-1)));setActive(next);popup.current?.querySelectorAll('[role="option"]')[next]?.scrollIntoView({block:'nearest'})}else if(e.key==='Enter'&&filtered[active]){e.preventDefault();choose(filtered[active])}}}>
 <div className="picker-search"><Icon name="search" size={14}/><input ref={search} aria-label="Search routes" placeholder="Find path, handler or package…" value={query} onChange={e=>{setQuery(e.target.value);setActive(0)}} aria-controls={id} aria-activedescendant={filtered[active]?`${id}-${active}`:undefined}/><span>esc</span></div>
 <div id={id} role="listbox" aria-label="Available routes" className="picker-options">{filtered.map((n,i)=><button key={n.id} id={`${id}-${i}`} role="option" aria-selected={n.id===value?.id} tabIndex={-1} className={active===i?'active':''} onPointerMove={()=>setActive(i)} onClick={()=>choose(n)}>{content(n)}{n.id===value?.id&&<Icon name="check" size={13}/>}</button>)}{!filtered.length&&<p className="picker-empty">No matching routes</p>}</div><div className="picker-footer">{filtered.length} routes<span>↑ ↓ to navigate · Enter to select</span></div>
 </div>,document.body)}</>
}
