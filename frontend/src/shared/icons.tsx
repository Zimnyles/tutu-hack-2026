import {
  ArrowUpRight, Award, Bus, Building2, Calendar, Cloud, Coins, Compass, Crown, Flag, Gem, Landmark,
  Layers, Leaf, Map, Moon, Mountain, PartyPopper, Plane, Route, Shield, Sparkles, Star, Target,
  Ticket, TrainFront, TramFront, Trophy, UsersRound, Utensils, Waves, Zap, type LucideIcon,
} from 'lucide-react'

const glyphs: Record<string, LucideIcon> = {
  arrow: ArrowUpRight,
  avia: Plane,
  bolt: Zap,
  building: Building2,
  bus: Bus,
  calendar: Calendar,
  calm: Cloud,
  cloud: Cloud,
  coins: Coins,
  compass: Compass,
  crown: Crown,
  diamond: Gem,
  etrain: TramFront,
  flag: Flag,
  food: Utensils,
  gem: Gem,
  layers: Layers,
  leaf: Leaf,
  map: Map,
  moon: Moon,
  mountain: Mountain,
  museum: Landmark,
  party: PartyPopper,
  people: UsersRound,
  plane: Plane,
  railway: TrainFront,
  route: Route,
  shield: Shield,
  spark: Sparkles,
  star: Star,
  target: Target,
  ticket: Ticket,
  train: TrainFront,
  trophy: Trophy,
  users: UsersRound,
  wave: Waves,
  award: Award,
}

export function Glyph({ code }: { code?: string }) {
  const Icon = glyphs[code ?? ''] ?? Sparkles

  return <Icon aria-hidden="true" />
}
