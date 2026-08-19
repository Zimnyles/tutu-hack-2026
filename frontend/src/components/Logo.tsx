export function Logo({ inverse = false }: { inverse?: boolean }) {
  return (
    <div className="logo" aria-label="Открывай от Туту">
      <img src={inverse ? '/brand/tutu-white.svg' : '/brand/tutu-lilac.svg'} alt="Туту" />
      <span className="logo-product">Открывай</span>
    </div>
  )
}
