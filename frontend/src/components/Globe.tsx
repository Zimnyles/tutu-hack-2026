import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { ChevronDown, ChevronLeft, ChevronRight, ChevronUp, Crosshair, Minus, Plus, X } from 'lucide-react'
import * as THREE from 'three'
import type { Territory } from '../types'

const GLOBE_RADIUS = 2
const ATMOSPHERE_SCALE = 1.07
const MIN_ZOOM = GLOBE_RADIUS * ATMOSPHERE_SCALE + 0.45
const MAX_ZOOM = 9
const UPCOMING_EVENT_COLOR = 0x2f7bff
const MAX_PITCH = 0.92
const REFERENCE_DISTANCE = 6.8
const MAX_MARKER_SCALE = 1.15
const MARKER_SIZE = 0.04
const MARKER_SIZE_HIGHLIGHTED = 0.05
const MARKER_SIZE_LOCKED = 0.03
const MARKER_HOVER_SCALE = 1.6
const PULSE_PERIOD = 1.9
const PULSE_SPREAD = 1.6
const PULSE_OPACITY = 0.55
const PICK_RADIUS = 22
const TAP_TOLERANCE = 8
const ROTATE_STEP = 0.18
const ZOOM_STEP = 0.3
const HOLD_INTERVAL = 120

type GlobeProps = {
  territories: Territory[]
  onSelect: (territory: Territory) => void
  homeCityID?: string
  reduceMotion?: boolean
}

type GlobeControls = {
  rotate: (yaw: number, pitch: number) => void
  zoom: (delta: number) => void
  reset: () => void
}

type HoveredMarker = {
  id: string
  name: string
  region: string
  state: Territory['state']
  events: number
  popular: boolean
  promoPercent: number
  x: number
  y: number
}

type MarkerCluster = {
  x: number
  y: number
  items: HoveredMarker[]
}

const stateLabels: Record<Territory['state'], string> = {
  locked: 'Не открыто',
  suggested: 'Рекомендуем',
  planned: 'В планах',
  arrived: 'Открыто',
}

function globePoint(latitude: number, longitude: number, radius = GLOBE_RADIUS) {
  const latitudeAngle = (90 - latitude) * Math.PI / 180
  const longitudeAngle = (longitude + 180) * Math.PI / 180

  return new THREE.Vector3(
    -radius * Math.sin(latitudeAngle) * Math.cos(longitudeAngle),
    radius * Math.cos(latitudeAngle),
    radius * Math.sin(latitudeAngle) * Math.sin(longitudeAngle),
  )
}

function markerColor(territory: Territory) {
  if (hasUpcomingEvents(territory)) return UPCOMING_EVENT_COLOR
  if (territory.state === 'arrived') return 0x00c95e
  if (territory.state === 'suggested') return 0xff872e
  if (territory.state === 'planned') return 0x5f94ff

  return 0xff86f8
}

function canvasTexture(draw: (context: CanvasRenderingContext2D, size: number) => void) {
  const size = 128
  const canvas = document.createElement('canvas')
  canvas.width = size
  canvas.height = size

  const context = canvas.getContext('2d')
  if (!context) return null

  draw(context, size)

  const texture = new THREE.CanvasTexture(canvas)
  texture.colorSpace = THREE.SRGBColorSpace

  return texture
}

// Белая заливка тонируется цветом состояния; отступ по краю оставлен на сглаживание.
function dotTexture() {
  return canvasTexture((context, size) => {
    const center = size / 2

    context.beginPath()
    context.arc(center, center, center - 6, 0, Math.PI * 2)
    context.fillStyle = '#ffffff'
    context.fill()
  })
}

function pulseTexture() {
  return canvasTexture((context, size) => {
    const center = size / 2

    context.beginPath()
    context.arc(center, center, center - 9, 0, Math.PI * 2)
    context.lineWidth = 12
    context.strokeStyle = '#ffffff'
    context.stroke()
  })
}

function hasUpcomingEvents(territory: Territory) {
  return Boolean(territory.popular_event)
}

function eventsWord(count: number) {
  const tail = count % 100
  if (tail > 10 && tail < 20) return 'событий'

  const last = count % 10
  if (last === 1) return 'событие'
  if (last >= 2 && last <= 4) return 'события'

  return 'событий'
}

function homeOrientation(territories: Territory[]) {
  if (territories.length === 0) return { yaw: 0, pitch: 0 }

  const center = territories.reduce(
    (total, territory) => total.add(globePoint(territory.latitude, territory.longitude, 1)),
    new THREE.Vector3(),
  ).divideScalar(territories.length).normalize()

  return {
    yaw: Math.atan2(-center.x, center.z),
    pitch: THREE.MathUtils.clamp(Math.atan2(center.y, Math.hypot(center.x, center.z)), -0.62, 0.62),
  }
}

export function Globe({ territories, onSelect, homeCityID = '', reduceMotion = false }: GlobeProps) {
  const host = useRef<HTMLDivElement>(null)
  const controls = useRef<GlobeControls | null>(null)
  const hoveredID = useRef<string | null>(null)
  const [fallback, setFallback] = useState(false)
  const [hovered, setHovered] = useState<HoveredMarker | null>(null)
  const [cluster, setCluster] = useState<MarkerCluster | null>(null)

  const clearHover = useCallback(() => {
    hoveredID.current = null
    setHovered(null)
  }, [])

  useEffect(() => {
    if (!cluster) return

    const close = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setCluster(null)
    }

    window.addEventListener('keydown', close)

    return () => window.removeEventListener('keydown', close)
  }, [cluster])

  useEffect(() => {
    if (!host.current) return

    let renderer: THREE.WebGLRenderer
    try {
      renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, powerPreference: 'high-performance' })
    } catch {
      setFallback(true)
      return
    }

    const root = host.current
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
    renderer.setSize(root.clientWidth, root.clientHeight)
    renderer.setClearColor(0x000000, 0)
    renderer.outputColorSpace = THREE.SRGBColorSpace
    renderer.toneMapping = THREE.ACESFilmicToneMapping
    renderer.toneMappingExposure = 1.08
    root.appendChild(renderer.domElement)

    const scene = new THREE.Scene()
    const camera = new THREE.PerspectiveCamera(38, root.clientWidth / root.clientHeight, 0.1, 100)
    camera.position.set(0, 0.25, 6.8)

    scene.add(new THREE.HemisphereLight(0xd8e4ff, 0x090733, 2.2))

    const sunlight = new THREE.DirectionalLight(0xffffff, 3.4)
    sunlight.position.set(-3, 4, 5)
    scene.add(sunlight)

    const brandRim = new THREE.PointLight(0xff45e3, 20, 12)
    brandRim.position.set(4, -2, -2)
    scene.add(brandRim)

    const group = new THREE.Group()
    scene.add(group)

    const home = homeOrientation(territories)
    const view = { yaw: home.yaw, pitch: home.pitch, distance: camera.position.z }
    const target = { ...view }

    let disposed = false
    const textureLoader = new THREE.TextureLoader()
    const maxAnisotropy = renderer.capabilities.getMaxAnisotropy()
    const loadedTextures: THREE.Texture[] = []

    const prepareTexture = (texture: THREE.Texture, srgb: boolean) => {
      if (srgb) texture.colorSpace = THREE.SRGBColorSpace
      texture.anisotropy = maxAnisotropy
      texture.generateMipmaps = true
      texture.minFilter = THREE.LinearMipmapLinearFilter
      texture.magFilter = THREE.LinearFilter
      loadedTextures.push(texture)

      return texture
    }

    const earthMaterial = new THREE.MeshStandardMaterial({
      map: prepareTexture(textureLoader.load('/textures/earth-color-2k.jpg'), true),
      normalMap: prepareTexture(textureLoader.load('/textures/earth-normal-2k.jpg'), false),
      normalScale: new THREE.Vector2(0.65, 0.65),
      roughnessMap: prepareTexture(textureLoader.load('/textures/earth-roughness-2k.jpg'), false),
      roughness: 1,
      metalness: 0.04,
      emissive: 0x070621,
      emissiveIntensity: 0.1,
    })

    textureLoader.load('/textures/earth-color-4k.jpg', (texture) => {
      if (disposed) {
        texture.dispose()

        return
      }

      earthMaterial.map = prepareTexture(texture, true)
      earthMaterial.needsUpdate = true
    })

    const earth = new THREE.Mesh(new THREE.SphereGeometry(GLOBE_RADIUS, 160, 120), earthMaterial)
    group.add(earth)

    const brandWash = new THREE.Mesh(
      new THREE.SphereGeometry(GLOBE_RADIUS * 1.002, 96, 64),
      new THREE.MeshBasicMaterial({
        color: 0x7d71ff,
        transparent: true,
        opacity: 0.06,
        blending: THREE.AdditiveBlending,
      }),
    )
    group.add(brandWash)

    const grid = new THREE.Mesh(
      new THREE.SphereGeometry(GLOBE_RADIUS * 1.005, 48, 32),
      new THREE.MeshBasicMaterial({ color: 0xedefff, wireframe: true, transparent: true, opacity: 0.04 }),
    )
    group.add(grid)

    const atmosphere = new THREE.Mesh(
      new THREE.SphereGeometry(GLOBE_RADIUS * ATMOSPHERE_SCALE, 64, 64),
      new THREE.MeshBasicMaterial({
        color: 0x7d71ff,
        transparent: true,
        opacity: 0.18,
        side: THREE.BackSide,
        blending: THREE.AdditiveBlending,
      }),
    )
    group.add(atmosphere)

    const dot = dotTexture()
    const ring = pulseTexture()
    const markers: Array<{
      territory: Territory
      anchor: THREE.Vector3
      sprite: THREE.Sprite
      pulse: THREE.Sprite | null
      size: number
      scale: number
    }> = []
    const highlightedSuggestions = new Set(
      territories
        .filter((territory) => territory.state === 'suggested')
        .sort((left, right) => right.rarity - left.rarity || right.reward - left.reward)
        .slice(0, 16)
        .map((territory) => territory.id),
    )

    territories.forEach((territory) => {
      const highlighted = hasUpcomingEvents(territory) || highlightedSuggestions.has(territory.id)
      const size = highlighted
        ? MARKER_SIZE_HIGHLIGHTED
        : territory.state === 'locked' ? MARKER_SIZE_LOCKED : MARKER_SIZE
      const color = markerColor(territory)
      const anchor = globePoint(territory.latitude, territory.longitude, GLOBE_RADIUS + 0.035)

      const marker = new THREE.Sprite(
        new THREE.SpriteMaterial({
          map: dot,
          color,
          transparent: true,
          depthWrite: false,
          toneMapped: false,
        }),
      )
      marker.position.copy(anchor)
      marker.scale.setScalar(size)
      marker.renderOrder = 4
      group.add(marker)

      let pulse: THREE.Sprite | null = null
      if (highlighted) {
        pulse = new THREE.Sprite(
          new THREE.SpriteMaterial({
            map: ring,
            color,
            transparent: true,
            opacity: PULSE_OPACITY,
            depthWrite: false,
            toneMapped: false,
          }),
        )
        pulse.position.copy(anchor)
        pulse.scale.setScalar(size)
        pulse.renderOrder = 3
        group.add(pulse)
      }

      markers.push({ territory, anchor: anchor.clone(), sprite: marker, pulse, size, scale: 1 })
    })

    const openedTerritories = territories.filter((territory) => territory.state === 'arrived')
    const routeOrigin = openedTerritories.find((territory) => territory.id === homeCityID) ?? openedTerritories[0]
    if (routeOrigin) {
      openedTerritories.filter((territory) => territory.id !== routeOrigin.id).forEach((territory) => {
        const start = globePoint(routeOrigin.latitude, routeOrigin.longitude, GLOBE_RADIUS + 0.045)
        const end = globePoint(territory.latitude, territory.longitude, GLOBE_RADIUS + 0.045)
        const midpoint = start.clone().add(end).multiplyScalar(0.5).normalize().multiplyScalar(GLOBE_RADIUS + 0.55)
        const route = new THREE.QuadraticBezierCurve3(start, midpoint, end)
        const line = new THREE.Line(
          new THREE.BufferGeometry().setFromPoints(route.getPoints(48)),
          new THREE.LineBasicMaterial({ color: 0x00c95e, transparent: true, opacity: 0.72 }),
        )
        group.add(line)
      })
    }

    let dragDistance = 0
    let animationFrame = 0
    let pinchDistance = 0
    const activePointers = new Map<number, { x: number; y: number }>()
    const worldPosition = new THREE.Vector3()
    const projected = new THREE.Vector3()
    const cameraDirection = new THREE.Vector3()

    const dragging = () => activePointers.size > 0

    const pointerSpread = () => {
      const [first, second] = [...activePointers.values()]

      return Math.hypot(first.x - second.x, first.y - second.y)
    }

    const rotateBy = (yaw: number, pitch: number) => {
      target.yaw += yaw
      target.pitch = THREE.MathUtils.clamp(target.pitch + pitch, -MAX_PITCH, MAX_PITCH)
    }

    const zoomBy = (delta: number) => {
      target.distance = THREE.MathUtils.clamp(target.distance + delta, MIN_ZOOM, MAX_ZOOM)
    }

    controls.current = {
      rotate: rotateBy,
      zoom: zoomBy,
      reset: () => {
        target.yaw = home.yaw
        target.pitch = home.pitch
        target.distance = 6.8
      },
    }

    const markersAt = (clientX: number, clientY: number) => {
      const bounds = renderer.domElement.getBoundingClientRect()
      if (!bounds.width || !bounds.height) return []

      const localX = clientX - bounds.left
      const localY = clientY - bounds.top
      const horizon = GLOBE_RADIUS / camera.position.length()
      cameraDirection.copy(camera.position).normalize()
      group.updateMatrixWorld()

      const found: Array<HoveredMarker & { distance: number }> = []

      for (const { territory, anchor } of markers) {
        worldPosition.copy(anchor).applyMatrix4(group.matrixWorld)
        if (worldPosition.clone().normalize().dot(cameraDirection) < horizon) continue

        projected.copy(worldPosition).project(camera)
        const screenX = (projected.x + 1) / 2 * bounds.width
        const screenY = (1 - projected.y) / 2 * bounds.height
        const distance = Math.hypot(screenX - localX, screenY - localY)
        if (distance >= PICK_RADIUS) continue

        found.push({
          id: territory.id,
          name: territory.name,
          region: territory.region,
          state: territory.state,
          events: territory.upcoming_events ?? 0,
          popular: Boolean(territory.popular_event),
          promoPercent: territory.promo_percent ?? 0,
          x: screenX,
          y: screenY,
          distance,
        })
      }

      return found.sort((left, right) => left.distance - right.distance)
    }

    const markerAt = (clientX: number, clientY: number) => markersAt(clientX, clientY)[0] ?? null

    const handlePointerDown = (event: PointerEvent) => {
      activePointers.set(event.pointerId, { x: event.clientX, y: event.clientY })
      dragDistance = 0
      pinchDistance = activePointers.size === 2 ? pointerSpread() : 0
      clearHover()
      setCluster(null)
      renderer.domElement.setPointerCapture(event.pointerId)
    }

    const handlePointerMove = (event: PointerEvent) => {
      const previous = activePointers.get(event.pointerId)
      if (!previous) {
        if (event.pointerType === 'touch') return

        const found = markerAt(event.clientX, event.clientY)
        renderer.domElement.style.cursor = found ? 'pointer' : 'grab'
        if (found?.id !== hoveredID.current) {
          hoveredID.current = found?.id ?? null
          setHovered(found)
        }

        return
      }

      activePointers.set(event.pointerId, { x: event.clientX, y: event.clientY })

      if (activePointers.size === 2) {
        const spread = pointerSpread()
        if (pinchDistance > 0) {
          zoomBy((pinchDistance - spread) * 0.01)
        }
        pinchDistance = spread
        dragDistance = TAP_TOLERANCE

        return
      }

      const deltaX = event.clientX - previous.x
      const deltaY = event.clientY - previous.y
      dragDistance += Math.abs(deltaX) + Math.abs(deltaY)
      rotateBy(deltaX * 0.006, deltaY * 0.004)
    }

    const handlePointerUp = (event: PointerEvent) => {
      const wasPinching = activePointers.size > 1
      activePointers.delete(event.pointerId)
      pinchDistance = 0

      if (wasPinching || dragDistance >= TAP_TOLERANCE) return

      const found = markersAt(event.clientX, event.clientY)
      if (found.length === 0) return

      if (found.length > 1) {
        setCluster({ x: found[0].x, y: found[0].y, items: found })

        return
      }

      const territory = territories.find((item) => item.id === found[0].id)
      if (territory) onSelect(territory)
    }

    const handlePointerCancel = (event: PointerEvent) => {
      activePointers.delete(event.pointerId)
      pinchDistance = 0
    }

    const handlePointerLeave = () => clearHover()

    const handleWheel = (event: WheelEvent) => {
      event.preventDefault()
      clearHover()
      zoomBy(event.deltaY * 0.004)
    }

    renderer.domElement.style.cursor = 'grab'
    renderer.domElement.addEventListener('pointerdown', handlePointerDown)
    renderer.domElement.addEventListener('pointermove', handlePointerMove)
    renderer.domElement.addEventListener('pointerup', handlePointerUp)
    renderer.domElement.addEventListener('pointercancel', handlePointerCancel)
    renderer.domElement.addEventListener('pointerleave', handlePointerLeave)
    renderer.domElement.addEventListener('wheel', handleWheel, { passive: false })

    const clock = new THREE.Clock()
    const animate = () => {
      const elapsed = clock.getElapsedTime()
      const smoothing = reduceMotion ? 1 : dragging() ? 0.32 : 0.14

      view.yaw += (target.yaw - view.yaw) * smoothing
      view.pitch += (target.pitch - view.pitch) * smoothing
      view.distance += (target.distance - view.distance) * smoothing
      group.rotation.set(view.pitch, view.yaw, 0)
      camera.position.z = view.distance

      // Размер точки следует за дистанцией камеры, поэтому на экране она остаётся одинаковой,
      // а при приближении перестаёт закрывать карту.
      const zoomScale = Math.min(view.distance / REFERENCE_DISTANCE, MAX_MARKER_SCALE)

      markers.forEach((marker, index) => {
        const wanted = marker.territory.id === hoveredID.current ? MARKER_HOVER_SCALE : 1
        marker.scale += (wanted - marker.scale) * 0.2

        const size = marker.size * marker.scale * zoomScale
        marker.sprite.scale.setScalar(size)

        if (!marker.pulse) return

        if (reduceMotion) {
          marker.pulse.scale.setScalar(size * 1.5)
          marker.pulse.material.opacity = PULSE_OPACITY * 0.5

          return
        }

        const phase = ((elapsed + index * 0.21) % PULSE_PERIOD) / PULSE_PERIOD
        marker.pulse.scale.setScalar(size * (1 + phase * PULSE_SPREAD))
        marker.pulse.material.opacity = PULSE_OPACITY * (1 - phase)
      })

      renderer.render(scene, camera)
      animationFrame = requestAnimationFrame(animate)
    }
    animate()

    const resize = () => {
      if (!root.clientWidth || !root.clientHeight) return
      camera.aspect = root.clientWidth / root.clientHeight
      camera.updateProjectionMatrix()
      renderer.setSize(root.clientWidth, root.clientHeight)
    }
    const resizeObserver = new ResizeObserver(resize)
    resizeObserver.observe(root)

    return () => {
      disposed = true
      controls.current = null
      cancelAnimationFrame(animationFrame)
      resizeObserver.disconnect()
      renderer.domElement.removeEventListener('pointerdown', handlePointerDown)
      renderer.domElement.removeEventListener('pointermove', handlePointerMove)
      renderer.domElement.removeEventListener('pointerup', handlePointerUp)
      renderer.domElement.removeEventListener('pointercancel', handlePointerCancel)
      renderer.domElement.removeEventListener('pointerleave', handlePointerLeave)
      renderer.domElement.removeEventListener('wheel', handleWheel)
      loadedTextures.forEach((texture) => texture.dispose())
      dot?.dispose()
      ring?.dispose()
      scene.traverse((object) => {
        if (object instanceof THREE.Sprite) {
          object.material.dispose()

          return
        }

        if (object instanceof THREE.Mesh || object instanceof THREE.Line) {
          object.geometry.dispose()
          if (Array.isArray(object.material)) object.material.forEach((material) => material.dispose())
          else object.material.dispose()
        }
      })
      renderer.dispose()
      root.replaceChildren()
    }
  }, [clearHover, homeCityID, onSelect, reduceMotion, territories])

  if (fallback) {
    return (
      <div className="globe-fallback">
        <strong>Карта доступна списком</strong>
        {territories.map((territory) => (
          <button key={territory.id} onClick={() => onSelect(territory)}>
            {territory.name}
            <span>{territory.state === 'arrived' ? 'Открыто' : 'Не открыто'}</span>
          </button>
        ))}
      </div>
    )
  }

  return (
    <div className="globe-stage">
      <div
        className="globe-canvas"
        ref={host}
        role="img"
        aria-label="Глобус с городами. Перетаскивание и кнопки управления вращают глобус, щипок или колесо меняют масштаб. Для выбора с клавиатуры используйте кнопку «Города»."
      />
      {hovered && !cluster && (
        <div className="globe-tooltip" style={{ left: hovered.x, top: hovered.y }} aria-hidden="true">
          <strong>{hovered.name}</strong>
          <span>{hovered.region}</span>
          {hovered.popular ? (
            <em className="has-events">Популярное событие</em>
          ) : hovered.events > 0 ? (
            <em>Скоро {hovered.events} {eventsWord(hovered.events)}</em>
          ) : (
            <em className={hovered.state}>{stateLabels[hovered.state]}</em>
          )}
          {hovered.promoPercent > 0 && <b>Промокод −{hovered.promoPercent}% на билеты</b>}
        </div>
      )}
      {cluster && (
        <div
          className="globe-cluster"
          style={{ left: cluster.x, top: cluster.y }}
          role="group"
          aria-label="Города рядом"
        >
          <header>
            <strong>Города рядом</strong>
            <button type="button" onClick={() => setCluster(null)} aria-label="Закрыть список">
              <X aria-hidden="true" />
            </button>
          </header>
          {cluster.items.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => {
                const territory = territories.find((candidate) => candidate.id === item.id)
                setCluster(null)
                if (territory) onSelect(territory)
              }}
            >
              <span className={`state-dot ${item.popular ? 'has-events' : item.state}`} aria-hidden="true" />
              <span>
                <strong>{item.name}</strong>
                <small>{item.region}</small>
              </span>
            </button>
          ))}
        </div>
      )}
      <div className="globe-controls" role="group" aria-label="Управление камерой">
        <div className="globe-pad">
          <HoldButton
            className="pad-up"
            label="Повернуть глобус вверх"
            onTrigger={() => {
              clearHover()
              controls.current?.rotate(0, -ROTATE_STEP)
            }}
          >
            <ChevronUp aria-hidden="true" />
          </HoldButton>
          <HoldButton
            className="pad-left"
            label="Повернуть глобус влево"
            onTrigger={() => {
              clearHover()
              controls.current?.rotate(-ROTATE_STEP, 0)
            }}
          >
            <ChevronLeft aria-hidden="true" />
          </HoldButton>
          <button
            type="button"
            className="pad-reset"
            aria-label="Вернуть исходный вид"
            onClick={() => {
              clearHover()
              controls.current?.reset()
            }}
          >
            <Crosshair aria-hidden="true" />
          </button>
          <HoldButton
            className="pad-right"
            label="Повернуть глобус вправо"
            onTrigger={() => {
              clearHover()
              controls.current?.rotate(ROTATE_STEP, 0)
            }}
          >
            <ChevronRight aria-hidden="true" />
          </HoldButton>
          <HoldButton
            className="pad-down"
            label="Повернуть глобус вниз"
            onTrigger={() => {
              clearHover()
              controls.current?.rotate(0, ROTATE_STEP)
            }}
          >
            <ChevronDown aria-hidden="true" />
          </HoldButton>
        </div>
        <div className="globe-zoom">
          <HoldButton
            label="Приблизить"
            onTrigger={() => {
              clearHover()
              controls.current?.zoom(-ZOOM_STEP)
            }}
          >
            <Plus aria-hidden="true" />
          </HoldButton>
          <HoldButton
            label="Отдалить"
            onTrigger={() => {
              clearHover()
              controls.current?.zoom(ZOOM_STEP)
            }}
          >
            <Minus aria-hidden="true" />
          </HoldButton>
        </div>
      </div>
    </div>
  )
}

function HoldButton({
  label,
  className,
  onTrigger,
  children,
}: {
  label: string
  className?: string
  onTrigger: () => void
  children: ReactNode
}) {
  const timer = useRef<number | null>(null)

  const stop = useCallback(() => {
    if (timer.current !== null) {
      window.clearInterval(timer.current)
      timer.current = null
    }
  }, [])

  useEffect(() => stop, [stop])

  return (
    <button
      type="button"
      className={className}
      aria-label={label}
      onPointerDown={(event) => {
        event.preventDefault()
        onTrigger()
        stop()
        timer.current = window.setInterval(onTrigger, HOLD_INTERVAL)
      }}
      onPointerUp={stop}
      onPointerLeave={stop}
      onPointerCancel={stop}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') onTrigger()
      }}
    >
      {children}
    </button>
  )
}
