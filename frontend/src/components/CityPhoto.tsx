import { useState } from 'react'
import { useCityPhoto } from '../shared/useCityPhoto'

type CityPhotoProps = {
  name: string
  className?: string
  overlayClassName?: string
  size?: number
}

export function CityPhoto({ name, className, overlayClassName, size }: CityPhotoProps) {
  const photo = useCityPhoto(name, size)
  const [brokenSource, setBrokenSource] = useState('')

  const source = photo.data
  if (!source || source === brokenSource) {
    return null
  }

  return (
    <>
      <img
        className={className}
        src={source}
        alt={`Фотография ${name}`}
        loading="lazy"
        decoding="async"
        referrerPolicy="no-referrer"
        onError={() => setBrokenSource(source)}
      />
      {overlayClassName && <span className={overlayClassName} aria-hidden="true" />}
    </>
  )
}
