import { useEffect, useMemo, useState } from 'react'
import { Button } from '@/components/ui/button'

// Layer: default (apples + snakes)
function layerDefault(key, { snakeMap }) {
  const snake = snakeMap[key]

  const content = snake ? (
    <div
      className={`rounded-full ${
        snake.owner === 0 ? 'bg-purple-500' : 'bg-green-500'
      } ${snake.isHead ? 'w-[80%] h-[80%]' : 'w-[60%] h-[60%]'}`}
    />
  ) : null

  return { bg: '', content, outline: null }
}

// Layer: surfaces
function layerSurfaces(key, { surfMap, linkPathSet, appleLinkPathSet, appleLinkDotSet }) {
  const surfId = surfMap[key]
  const isSurf = surfId !== undefined
  const isPath = linkPathSet.has(key)
  const isAppleLinkPath = appleLinkPathSet.has(key)
  const isAppleLinkDot = appleLinkDotSet.has(key)

  const outline = isSurf
    ? { outline: '2px dashed rgba(168,85,247,0.7)', outlineOffset: '-2px' }
    : null

  const content = (
    <>
      {isSurf && (
        <span className="absolute inset-0 flex items-center justify-center text-[7px] font-bold text-purple-400 pointer-events-none z-10">
          {surfId}
        </span>
      )}
      {isPath && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-20">
          <div className="w-[40%] h-[40%] rounded-full bg-orange-400 opacity-80" />
        </div>
      )}
      {isAppleLinkPath && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-30">
          <div className="w-[28%] h-[28%] rounded-full bg-red-500 opacity-80" />
        </div>
      )}
      {isAppleLinkDot && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-40">
          <div className="w-[58%] h-[58%] rounded-full border-2 border-red-600 bg-red-300/80" />
        </div>
      )}
    </>
  )

  return { bg: '', content, outline }
}

// Cluster color palette: 10 distinct hues
const clusterColors = [
  'hsl(0,70%,55%)', 'hsl(36,70%,55%)', 'hsl(72,70%,55%)',
  'hsl(108,70%,55%)', 'hsl(144,70%,55%)', 'hsl(180,70%,55%)',
  'hsl(216,70%,55%)', 'hsl(252,70%,55%)', 'hsl(288,70%,55%)',
  'hsl(324,70%,55%)',
]

// Layer: clusters
function layerClusters(key, { snakeMap, clusterMap, clusterData }) {
  const snake = snakeMap[key]
  const cid = clusterMap[key]

  let bg = ''
  let outline = null
  let label = null

  if (cid !== undefined && cid >= 0) {
    const cl = clusterData[cid]
    const color = cl && cl.size > 1 ? clusterColors[cid % clusterColors.length] : '#888'
    outline = { outline: `3px solid ${color}`, outlineOffset: '-2px' }
    label = (
      <span className="absolute inset-0 flex flex-col items-center justify-center text-[7px] font-bold pointer-events-none z-10 leading-tight" style={{ color }}>
        <span>C{cid}</span>
        {cl && <span className="text-[6px] opacity-70">×{cl.size}</span>}
      </span>
    )
  }

  const content = (
    <>
      {snake && (
        <div
          className={`rounded-full ${
            snake.owner === 0 ? 'bg-purple-500' : 'bg-green-500'
          } ${snake.isHead ? 'w-[80%] h-[80%]' : 'w-[60%] h-[60%]'}`}
        />
      )}
      {label}
    </>
  )

  return { bg, content, outline }
}

// Layer: heat map
function layerHeat(key, { snakeMap, heatMap }) {
  const snake = snakeMap[key]
  const heat = heatMap[key]

  let bg = ''
  let label = null
  if (heat !== undefined) {
    const { heat: h, myDist, opDist } = heat
    // Color: green = ours, red = theirs, yellow = contested
    if (h >= 9999) {
      bg = 'bg-green-400/70' // exclusive ours
    } else if (h <= -9999) {
      bg = 'bg-red-400/70' // unreachable
    } else if (h > 0) {
      bg = 'bg-green-300/60' // we're faster
    } else if (h < 0) {
      bg = 'bg-red-300/60' // they're faster
    } else {
      bg = 'bg-yellow-300/60' // tied
    }
    const myLabel = myDist >= 0 ? myDist : '-'
    const opLabel = opDist >= 0 ? opDist : '-'
    label = (
      <span className="absolute inset-0 flex flex-col items-center justify-center text-[7px] font-bold text-black pointer-events-none z-10 leading-tight">
        <span>{h >= 9999 ? '++' : h <= -9999 ? '--' : h > 0 ? `+${h}` : `${h}`}</span>
        <span className="text-[6px] opacity-70">{myLabel}/{opLabel}</span>
      </span>
    )
  }

  const content = (
    <>
      {snake && (
        <div
          className={`rounded-full ${
            snake.owner === 0 ? 'bg-purple-500' : 'bg-green-500'
          } ${snake.isHead ? 'w-[80%] h-[80%]' : 'w-[60%] h-[60%]'}`}
        />
      )}
      {label}
    </>
  )

  return { bg, content, outline: null }
}

// Layer: BFS paths
function layerBFS(key, { snakeMap, bfsHeadSet, bfsLandingSet, bfsAppleSet }) {
  const snake = snakeMap[key]
  const isHead = bfsHeadSet.has(key)
  const isLanding = bfsLandingSet.has(key)
  const isApple = bfsAppleSet.has(key)

  const content = (
    <>
      {snake && (
        <div
          className={`rounded-full ${
            snake.owner === 0 ? 'bg-purple-500' : 'bg-green-500'
          } ${snake.isHead ? 'w-[80%] h-[80%]' : 'w-[60%] h-[60%]'}`}
        />
      )}
      {isHead && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-20">
          <div className="w-[50%] h-[50%] rounded-full bg-cyan-400 opacity-90" />
        </div>
      )}
      {isLanding && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-30">
          <div className="w-[60%] h-[60%] rounded-full border-2 border-cyan-300 bg-cyan-500/60" />
        </div>
      )}
      {isApple && (
        <div className="absolute inset-0 flex items-center justify-center pointer-events-none z-40">
          <div className="w-[58%] h-[58%] rounded-full border-2 border-yellow-400 bg-yellow-300/70" />
        </div>
      )}
    </>
  )

  return { bg: '', content, outline: null }
}

export default function App() {
  const [data, setData] = useState(null)
  const [cursor, setCursor] = useState(null)
  const [pinnedCell, setPinnedCell] = useState(null)
  const [activeLayer, setActiveLayer] = useState(null) // null | 'surfaces' | 'bfs' | 'heat'
  const toggleLayer = (name) => setActiveLayer((prev) => prev === name ? null : name)
  const showSurfaces = activeLayer === 'surfaces'
  const showBFS = activeLayer === 'bfs'
  const showHeat = activeLayer === 'heat'

  const activeCell = pinnedCell || cursor

  useEffect(() => {
    fetch('/map.json')
      .then((r) => r.json())
      .then(setData)
  }, [])

  useEffect(() => {
    if (!pinnedCell) return
    const onKey = (e) => { if (e.key === 'Escape') setPinnedCell(null) }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [pinnedCell])

  const { w, h, walls, apples, snakes, surfaces, wallSet, appleSet, snakeMap, surfMap, snakeCellToId, heatMap, clusterMap, clusterData } = useMemo(() => {
    if (!data) return {}
    const { w, h, walls, apples, snakes, surfaces } = data
    const wallSet = new Set(walls.map((c) => `${c.x},${c.y}`))
    const appleSet = new Set(apples.map((c) => `${c.x},${c.y}`))
    const snakeMap = {}
    const snakeCellToId = {}
    for (const sn of snakes) {
      for (let i = 0; i < sn.body.length; i++) {
        const c = sn.body[i]
        const key = `${c.x},${c.y}`
        snakeMap[key] = { owner: sn.owner, isHead: i === 0, id: sn.id }
        snakeCellToId[key] = sn.id
      }
    }
    const surfMap = {}
    for (const s of surfaces) {
      for (let x = s.left; x <= s.right; x++) {
        surfMap[`${x},${s.y}`] = s.id
      }
    }
    const heatMap = {}
    for (const a of apples) {
      heatMap[`${a.x},${a.y}`] = { heat: a.heat, myDist: a.myDist, opDist: a.opDist }
    }
    const clusterMap = {}
    for (const a of apples) {
      if (a.clusterId >= 0) clusterMap[`${a.x},${a.y}`] = a.clusterId
    }
    const clusterData = {}
    for (const c of (data.clusters ?? [])) {
      clusterData[c.id] = c
    }
    return { w, h, walls, apples, snakes, surfaces, wallSet, appleSet, snakeMap, surfMap, snakeCellToId, heatMap, clusterMap, clusterData }
  }, [data])

  if (!data) return <div className="p-4 font-mono">Loading...</div>

  // Find active snake (hovered/pinned cell is on a snake)
  const activeSnake = activeCell ? snakes.find(sn => snakeCellToId[`${activeCell.x},${activeCell.y}`] === sn.id) : null

  // Build link path overlay for hovered surface
  const linkPathSet = new Set()
  const appleLinkPathSet = new Set()
  const appleLinkDotSet = new Set()
  if (showSurfaces && activeCell) {
    const sid = surfMap[`${activeCell.x},${activeCell.y}`]
    if (sid !== undefined) {
      const s = surfaces[sid]
      for (const l of s.links) {
        for (const p of l.path) {
          linkPathSet.add(`${p.x},${p.y}`)
        }
      }
      for (const l of s.appleLinks ?? []) {
        for (const p of l.path) {
          appleLinkPathSet.add(`${p.x},${p.y}`)
        }
        appleLinkDotSet.add(`${l.apple.x},${l.apple.y}`)
      }
    }
  }

  // Build BFS overlay for hovered snake
  const bfsHeadSet = new Set()
  const bfsLandingSet = new Set()
  const bfsAppleSet = new Set()
  if (showBFS && activeSnake?.plan) {
    const plan = activeSnake.plan
    // Show all surf reach head traces
    for (const sr of plan.surfReaches ?? []) {
      for (const h of sr.heads) {
        bfsHeadSet.add(`${h.x},${h.y}`)
      }
      bfsLandingSet.add(`${sr.landing.x},${sr.landing.y}`)
    }
    // Show transit heads
    if (plan.transit) {
      for (const h of plan.transit.heads) {
        bfsHeadSet.add(`${h.x},${h.y}`)
      }
      bfsLandingSet.add(`${plan.transit.landing.x},${plan.transit.landing.y}`)
    }
    // Show best apple target
    if (plan.bestApple) {
      bfsAppleSet.add(`${plan.bestApple.x},${plan.bestApple.y}`)
    }
    // Show all reachable apples
    for (const a of plan.apples ?? []) {
      bfsAppleSet.add(`${a.apple.x},${a.apple.y}`)
    }
  }

  // Pick active layer
  const showClusters = activeLayer === 'clusters'
  const layerCtx = { appleSet, snakeMap, surfMap, linkPathSet, appleLinkPathSet, appleLinkDotSet, bfsHeadSet, bfsLandingSet, bfsAppleSet, heatMap, clusterMap, clusterData }
  const renderLayer = showClusters ? layerClusters : showHeat ? layerHeat : showBFS ? layerBFS : showSurfaces ? layerSurfaces : layerDefault

  const cellHandlers = (x, y) => ({
    onMouseEnter: () => setCursor({ x, y }),
    onMouseLeave: () => setCursor(null),
    onClick: (e) => { e.stopPropagation(); setPinnedCell((prev) => prev ? null : { x, y }) },
  })

  const activeSurfId = activeCell ? surfMap[`${activeCell.x},${activeCell.y}`] : undefined
  const activeSurf = activeSurfId !== undefined ? surfaces[activeSurfId] : null

  return (
    <div className="flex gap-2 p-2 font-mono h-screen">
      <div className="flex-1 min-w-0 flex items-center justify-center h-full overflow-hidden p-1">
        <div className="h-full" style={{ aspectRatio: `${w} / ${h}`, maxWidth: '100%' }}>
        <table
          className="border-collapse w-full h-full"
        >
          <tbody>
            <tr>
              <td />
              {Array.from({ length: w }, (_, x) => (
                <td key={x} className="text-[9px] text-muted-foreground text-center" style={{ padding: 0, paddingBottom: 1 }}>
                  {x}
                </td>
              ))}
            </tr>
            {Array.from({ length: h }, (_, y) => (
              <tr key={y}>
                <td className="text-[9px] text-muted-foreground align-middle text-right" style={{ padding: 0, paddingRight: 2 }}>
                  {y}
                </td>
                {Array.from({ length: w }, (_, x) => {
                  const key = `${x},${y}`
                  const isWall = wallSet.has(key)

                  if (isWall) {
                    return (
                      <td
                        key={x}
                        className="border border-gray-300 bg-blue-500"
                        style={{ width: `${100 / w}%`, padding: 0 }}
                        {...cellHandlers(x, y)}
                      >
                        <div className="w-full h-full aspect-square" />
                      </td>
                    )
                  }

                  const isApple = appleSet.has(key)
                  const { bg, content, outline } = renderLayer(key, layerCtx)

                  return (
                    <td
                      key={x}
                      className={`border border-gray-300 relative ${bg}`}
                      style={{
                        width: `${100 / w}%`,
                        padding: 0,
                        ...(outline || {}),
                        ...(isApple ? { backgroundColor: 'rgba(250, 204, 21, 0.5)' } : {}),
                      }}
                      {...cellHandlers(x, y)}
                    >
                      <div className="w-full h-full flex items-center justify-center aspect-square">
                        {content}
                      </div>
                    </td>
                  )
                })}
                <td className="pl-1 text-[9px] text-muted-foreground align-middle" style={{ padding: 0, paddingLeft: 2 }}>
                  {y}
                </td>
              </tr>
            ))}
            <tr>
              <td />
              {Array.from({ length: w }, (_, x) => (
                <td key={x} className="text-[9px] text-muted-foreground text-center" style={{ padding: 0, paddingTop: 1 }}>
                  {x}
                </td>
              ))}
            </tr>
          </tbody>
        </table>
        </div>
      </div>
      <div className="min-w-0 w-[20%] shrink pt-4 text-sm overflow-y-auto overflow-x-hidden">
        <p>current: {cursor ? `${cursor.x},${cursor.y}` : '-'}</p>
        <p>w: {w} h: {h}</p>
        <p>walls: {walls.length} apples: {apples.length}</p>
        <p>snakes: {snakes.length}</p>
        <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-blue-500" /> wall</div>
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-yellow-400" /> apple</div>
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-purple-500 rounded-full" /> p0</div>
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-green-500 rounded-full" /> p1</div>
          {showBFS && <>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-cyan-400 rounded-full" /> head trace</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 rounded-full border-2 border-cyan-300 bg-cyan-500/60" /> landing</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 rounded-full border-2 border-yellow-400 bg-yellow-300/70" /> target apple</div>
          </>}
          {showClusters && <>
            <div className="flex items-center gap-1"><div className="w-3 h-3 border-2 border-red-500" /> cluster (colored by ID)</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 border-2 border-gray-400" /> isolated (size=1)</div>
          </>}
          {showHeat && <>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-green-400/70" /> exclusive ours</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-green-300/60" /> we're faster</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-yellow-300/60" /> tied</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-red-300/60" /> they're faster</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-red-400/70" /> unreachable</div>
          </>}
          {showSurfaces && <>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-orange-400 rounded-full" /> surface path</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 bg-red-500 rounded-full" /> apple path</div>
            <div className="flex items-center gap-1"><div className="w-3 h-3 rounded-full border-2 border-red-600 bg-red-300/80" /> apple target</div>
          </>}
        </div>
        <div className="mt-3 flex gap-2">
          {['surfaces', 'bfs', 'heat', 'clusters'].map((name) => (
            <Button
              key={name}
              variant={activeLayer === name ? 'default' : 'outline'}
              size="sm"
              className="cursor-pointer"
              onClick={() => toggleLayer(name)}
            >
              {name}
            </Button>
          ))}
        </div>
        {activeCell && (
          <div className="mt-4 border-t pt-2">
            <p>cell: {activeCell.x},{activeCell.y}</p>
            {!showSurfaces && !showBFS && activeSnake && (
              <div className="mt-2">
                <p className={activeSnake.owner === 0 ? 'text-purple-400' : 'text-green-400'}>
                  snake #{activeSnake.id} len: {activeSnake.body.length} head: ({activeSnake.body[0].x},{activeSnake.body[0].y})
                </p>
              </div>
            )}
            {showSurfaces && activeSurf && (() => {
              const s = activeSurf
              return (
                <div className="mt-2 text-purple-400">
                  <p className="font-bold">surface #{s.id}</p>
                  <p>y: {s.y} x: {s.left}..{s.right} len: {s.len}</p>
                  <p>edges: ({s.left},{s.y}){s.len > 1 ? ` (${s.right},${s.y})` : ''}</p>
                  <p>links: {s.links.length}</p>
                  <p>apple links: {s.appleLinks?.length ?? 0}</p>
                  {s.links.length > 0 && (
                    <div className="mt-1 text-[11px]">
                      {s.links.map((l, i) => (
                        <p key={i}>→ S{l.to}({l.landing.x},{l.landing.y}) d={l.len} from ({l.path[0].x},{l.path[0].y})</p>
                      ))}
                    </div>
                  )}
                  {(s.appleLinks?.length ?? 0) > 0 && (
                    <div className="mt-2 text-[11px] text-red-400">
                      {s.appleLinks.map((l, i) => (
                        <p key={i}>• A({l.apple.x},{l.apple.y}) d={l.len} from ({l.start.x},{l.start.y})</p>
                      ))}
                    </div>
                  )}
                </div>
              )
            })()}
            {showClusters && activeCell && (() => {
              const cid = clusterMap[`${activeCell.x},${activeCell.y}`]
              if (cid === undefined) return null
              const cl = clusterData[cid]
              if (!cl) return null
              const color = cl.size > 1 ? clusterColors[cid % clusterColors.length] : '#888'
              return (
                <div className="mt-2" style={{ color }}>
                  <p className="font-bold">cluster #{cid}</p>
                  <p>size: {cl.size}</p>
                  <div className="mt-1 text-[11px]">
                    {cl.apples.map((a, i) => (
                      <span key={i}>({a.x},{a.y}){i < cl.apples.length - 1 ? ' ' : ''}</span>
                    ))}
                  </div>
                </div>
              )
            })()}
            {showHeat && activeCell && (() => {
              const h = heatMap[`${activeCell.x},${activeCell.y}`]
              if (!h) return null
              return (
                <div className="mt-2 text-green-400">
                  <p className="font-bold">apple ({activeCell.x},{activeCell.y})</p>
                  <p>heat: {h.heat >= 9999 ? '++' : h.heat <= -9999 ? '--' : h.heat}</p>
                  <p>my dist: {h.myDist >= 0 ? h.myDist : 'unreachable'}</p>
                  <p>op dist: {h.opDist >= 0 ? h.opDist : 'unreachable'}</p>
                </div>
              )
            })()}
            {showBFS && activeSnake && (() => {
              const sn = activeSnake
              const plan = sn.plan
              return (
                <div className="mt-2 text-cyan-400">
                  <p className="font-bold">snake #{sn.id} ({sn.owner === 0 ? 'mine' : 'enemy'})</p>
                  <p>dir: {sn.dir} sp: {sn.sp} len: {sn.body.length}</p>
                  {plan && (
                    <div className="mt-1">
                      <p>{plan.onSurface ? 'on surface' : 'off surface'}</p>
                      {plan.bestApple && (
                        <p className="mt-1 text-yellow-400">target: ({plan.bestApple.x},{plan.bestApple.y}) d={plan.bestDist}</p>
                      )}
                      {plan.conflicting && (
                        <p className="mt-1 text-red-400">CONFLICT with snake idx {plan.conflictWith}</p>
                      )}
                      {(plan.surfReaches?.length ?? 0) > 0 && (
                        <div className="mt-2 text-[11px]">
                          <p className="font-bold">all reachable surfaces:</p>
                          {plan.surfReaches.map((sr, i) => (
                            <p key={i}>S{sr.surfId} d={sr.dist} [{sr.dirs.join(' ')}] → ({sr.landing.x},{sr.landing.y})</p>
                          ))}
                        </div>
                      )}
                      {(plan.apples?.length ?? 0) > 0 && (
                        <div className="mt-2 text-[11px] text-yellow-400">
                          <p className="font-bold">reachable apples:</p>
                          {plan.apples.slice(0, 10).map((a, i) => (
                            <p key={i}>({a.apple.x},{a.apple.y}) d={a.dist}</p>
                          ))}
                          {plan.apples.length > 10 && <p>...+{plan.apples.length - 10} more</p>}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )
            })()}
          </div>
        )}
      </div>
    </div>
  )
}
