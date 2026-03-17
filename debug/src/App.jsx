import { useEffect, useState } from 'react'
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
function layerSurfaces(key, { surfMap, linkPathSet }) {
  const surfId = surfMap[key]
  const isSurf = surfId !== undefined
  const isPath = linkPathSet.has(key)

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
    </>
  )

  return { bg: '', content, outline }
}

export default function App() {
  const [data, setData] = useState(null)
  const [hover, setHover] = useState(null)
  const [showSurfaces, setShowSurfaces] = useState(false)

  useEffect(() => {
    fetch('/map.json')
      .then((r) => r.json())
      .then(setData)
  }, [])

  if (!data) return <div className="p-4 font-mono">Loading...</div>

  const { w, h, walls, apples, snakes, surfaces } = data

  // Build lookup maps
  const wallSet = new Set(walls.map((c) => `${c.x},${c.y}`))
  const appleSet = new Set(apples.map((c) => `${c.x},${c.y}`))

  const snakeMap = {}
  for (const sn of snakes) {
    for (let i = 0; i < sn.body.length; i++) {
      const c = sn.body[i]
      snakeMap[`${c.x},${c.y}`] = { owner: sn.owner, isHead: i === 0 }
    }
  }

  const surfMap = {}
  for (const s of surfaces) {
    for (let x = s.left; x <= s.right; x++) {
      surfMap[`${x},${s.y}`] = s.id
    }
  }

  // Build link path overlay for hovered surface
  const linkPathSet = new Set()
  if (showSurfaces && hover) {
    const sid = surfMap[`${hover.x},${hover.y}`]
    if (sid !== undefined) {
      const s = surfaces[sid]
      for (const l of s.links) {
        for (const p of l.path) {
          linkPathSet.add(`${p.x},${p.y}`)
        }
      }
    }
  }

  // Pick active layer
  const layerCtx = { appleSet, snakeMap, surfMap, linkPathSet }
  const renderLayer = showSurfaces ? layerSurfaces : layerDefault

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
                        onMouseEnter={() => setHover({ x, y })}
                        onMouseLeave={() => setHover(null)}
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
                      onMouseEnter={() => setHover({ x, y })}
                      onMouseLeave={() => setHover(null)}
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
        <p>w: {w} h: {h}</p>
        <p>walls: {walls.length} apples: {apples.length}</p>
        <p>snakes: {snakes.length}</p>
        <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1">
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-blue-500" /> wall</div>
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-yellow-400" /> apple</div>
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-purple-500 rounded-full" /> p0</div>
          <div className="flex items-center gap-1"><div className="w-3 h-3 bg-green-500 rounded-full" /> p1</div>
        </div>
        <Button
          variant={showSurfaces ? 'default' : 'outline'}
          size="sm"
          className="mt-3 cursor-pointer"
          onClick={() => setShowSurfaces(!showSurfaces)}
        >
          surfaces {showSurfaces ? 'on' : 'off'}
        </Button>
        {hover && (
          <div className="mt-4 border-t pt-2">
            <p>cell: {hover.x},{hover.y}</p>
            {showSurfaces && surfMap[`${hover.x},${hover.y}`] !== undefined && (() => {
              const s = surfaces[surfMap[`${hover.x},${hover.y}`]]
              return (
                <div className="mt-2 text-purple-400">
                  <p className="font-bold">surface #{s.id}</p>
                  <p>y: {s.y} x: {s.left}..{s.right} len: {s.len}</p>
                  <p>edges: ({s.left},{s.y}){s.len > 1 ? ` (${s.right},${s.y})` : ''}</p>
                  <p>links: {s.links.length}</p>
                  {s.links.length > 0 && (
                    <div className="mt-1 text-[11px]">
                      {s.links.map((l, i) => (
                        <p key={i}>→ S{l.to} d={l.len}</p>
                      ))}
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
