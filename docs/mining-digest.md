# Mining Digest — AtariAge 77 threads, indexed to principles & checks
Distilled index of the 77 mined AtariAge threads. Raw thread captures stay in the umbrella `reference/atariage/<topic>/` (provenance); this is the citable, self-contained takeaway map in the harness. Each row → the design-principles section / `pkg/design` function / technique candidate it feeds.
出典正本: `reference/atariage/MINED.csv`（77行）。詳細は各 `notes.ja.md`。

## Color — 色・パレット
| topic | technique | thread | feeds |
|---|---|---|---|
| [136221](https://forums.atariage.com/topic/136221) | `5cycle-color-cycling` | 136221 — 5 cycle in-Kernel Color cycling routine | §色 / 省サイクル色替え(doc) |
| [160655](https://forums.atariage.com/topic/160655) | `bg-pf-per-scanline` | Changing Background AND Playfield on a per-scanline basis | §作画craft / design.GradientSameHue |
| [176987](https://forums.atariage.com/topic/176987) | `interlaced-multicolored-playfield` | Interlaced Multicolored Playfield | §多重化 / design.InterlaceColorsSafe |
| [160140](https://forums.atariage.com/topic/160140) | `multi-colored-sprites` | Multi-colored Sprites | §色 / design.Hue,Luminance |
| [170018](https://forums.atariage.com/topic/170018) | `multiple-colors-per-scanline` | Multiple colors per scanline | §色 / design.MinColorBandWidthPx,CheckColorBands |
| [132561](https://forums.atariage.com/topic/132561) | `palettes-compared` | 2600 and 5200 palettes compared | §色 / design.WashoutRisk,HueName,GradientSameHue |
| [118495](https://forums.atariage.com/topic/118495) | `rgb-color-values` | 118495 — Atari 2600 RGB color values | §色 / palette_stella.go(正本) |
| [101581](https://forums.atariage.com/topic/101581) | `symbolic-color-names` | 101581 — Symbolic names for 2600 colors PAL/NTSC | §色 / design.Hue,Luminance,HueName |

## Sprite — スプライト・位置決め
| topic | technique | thread | feeds |
|---|---|---|---|
| [209137](https://forums.atariage.com/topic/209137) | `48px-positioning` | 48-pixel sprite positioning and optimisations | §スプライト / design.PositionSplit, pkg/sprite.SplitWide |
| [214174](https://forums.atariage.com/topic/214174) | `animated-48px-sprite-routine` | Animated 48 Pixel Sprite Routine | §スプライト / pkg/sprite.SplitWide, design.WalkFrame |
| [122181](https://forums.atariage.com/topic/122181) | `back-to-back-sprite-data` | Back-to-back sprite data | §スプライト / pkg/sprite |
| [326595](https://forums.atariage.com/topic/326595) | `couch-compliant-logo` | Couch Compliant Logo | §作画craft / サムネ可読性,2:1(doc/design.PixelAspectRatio) |
| [160265](https://forums.atariage.com/topic/160265) | `creative-use-of-the-missile-sprites` | Creative Use of the Missile Sprites | §スプライト / missile=線(doc) |
| [118687](https://forums.atariage.com/topic/118687) | `detailed-missile-sprite-drawing-trick` | Detailed Missile-as-Sprite Drawing Trick | §スプライト / missile=線(doc) |
| [106110](https://forums.atariage.com/topic/106110) | `drawing-wizard-sprite` | Need help drawing Wizard sprite | §作画craft / 8pxシルエット可読性(doc) |
| [169238](https://forums.atariage.com/topic/169238) | `free-sprites-for-the-taking` | Free Sprites for the Taking | §スプライト / 既製GRP流用(doc) |
| [126450](https://forums.atariage.com/topic/126450) | `horizontal-positioning` | Horizontal Positioning is killing me! | §スプライト / design.PositionSplit,CoarseIterations |
| [301861](https://forums.atariage.com/topic/301861) | `walk-cycle-two-frames` | Alternating between two frames when walking | §作画craft / design.WalkFrame |

## Text/HUD — テキスト/HUD/スコア
| topic | technique | thread | feeds |
|---|---|---|---|
| [180632](https://forums.atariage.com/topic/180632) | `32-character-text-display` | 32 Character Text Display | §PF(HUD) / design.MaxChars(TextVenetian) |
| [111910](https://forums.atariage.com/topic/111910) | `hi-res-title-screens` | "Hi-res" タイトル画面の作り方 | §PF(HUD) / design.MaxChars |
| [318349](https://forums.atariage.com/topic/318349) | `pf48-title-tool` | Splash-O-Matic 2600 | §PF(HUD) / 48pxタイトル(tool) |
| [213458](https://forums.atariage.com/topic/213458) | `six-digit-scores-in-the-atari-2600` | Six Digit Scores | §PF(HUD) / pkg/sprite.DigitFont, score6 |
| [197162](https://forums.atariage.com/topic/197162) | `text-hud-icons` | Text/HUD/Icons/Compression matters | §PF(HUD) / design.MaxChars |
| [169819](https://forums.atariage.com/topic/169819) | `the-titlescreen-kernel` | The Titlescreen Kernel | §PF(HUD) / design.MaxChars(Text48px) |
| [294306](https://forums.atariage.com/topic/294306) | `title-screen-opinion` | Title Screen Opinion | §作画craft / 字形誤読(doc) |
| [295637](https://forums.atariage.com/topic/295637) | `title-to-game-transition` | 295637 — Transitioning from Title Screen to Game Mode | §カーネル予算 / GameState(doc) |

## Multiplex — 多重化・フリッカー
| topic | technique | thread | feeds |
|---|---|---|---|
| [160537](https://forums.atariage.com/topic/160537) | `flicker-to-enhance-graphics` | 160537 — Using flicker to enhance graphics | §多重化 / design.NeedsFlicker(意図フリッカ) |
| [107063](https://forums.atariage.com/topic/107063) | `interlacing-multi-sprites` | Interlacing, Multi-sprites, and More | §多重化 / design.NeedsFlicker,FitsMultiSprite |

## Playfield — プレイフィールド・スクロール
| topic | technique | thread | feeds |
|---|---|---|---|
| [131319](https://forums.atariage.com/topic/131319) | `asymmetric-reflected-playfield` | Asymmetric reflected playfield | §PF / design.AsymRightWindow,FitsAsymRightWrite |
| [319884](https://forums.atariage.com/topic/319884) | `atari-background-builder` | atari-background-builder | §作画craft / design.BackgroundSpec.Feasible |
| [132116](https://forums.atariage.com/topic/132116) | `castlevania-port` | Porting Castlevania to the 2600 | §PF / design.AsymRightWindow(非対称高コスト) |
| [317208](https://forums.atariage.com/topic/317208) | `maze-wall-detection` | Software maze wall detection | 衝突/PF / ⑫a 壁判定(technique候補) |
| [333797](https://forums.atariage.com/topic/333797) | `oozy-maze-quest` | OOZY the GOO Slime Quest | 衝突/PF / 迷路ゲーム(部分採掘) |
| [224946](https://forums.atariage.com/topic/224946) | `smooth-scrolling-playfield` | Smooth Scrolling Playfield | §PF / design.ScrollScanlinesConstant |
| [331285](https://forums.atariage.com/topic/331285) | `tankmaze` | TankMaze / Minotaur | 衝突/PF / ⑫a 迷路(tankmaze GitHubソース有) |
| [200972](https://forums.atariage.com/topic/200972) | `tile-based-scrolling-engines` | Tile based Scrolling and expanding Engines | §PF / design.ScrollScanlinesConstant |
| [294094](https://forums.atariage.com/topic/294094) | `tile-character-graphics-engine` | Jentzsch/Davie '2600 tile/character graphics engine | §PF / タイル差分エンジン(technique候補) |
| [96777](https://forums.atariage.com/topic/96777) | `vertical-scrolling-questions` | Vertical Scrolling Questions | §PF / design.ScrollScanlinesConstant |

## Kernel — カーネル予算・最適化
| topic | technique | thread | feeds |
|---|---|---|---|
| [128139](https://forums.atariage.com/topic/128139) | `chimera-kernel-submissions` | Chimera Kernel Submissions | §カーネル予算 / design.LineBudget |
| [113254](https://forums.atariage.com/topic/113254) | `fast-divide-by-seven` | Fast divide by seven | §カーネル予算 / 除算最適化(doc) |
| [124320](https://forums.atariage.com/topic/124320) | `illegal-opcodes` | Illegal opcodes | §カーネル予算 / ISC/ISB(doc,要litmus) |
| [313777](https://forums.atariage.com/topic/313777) | `modular-kernel` | Tips and tricks for a modular kernel | §カーネル予算 / design.LineBudget |
| [298395](https://forums.atariage.com/topic/298395) | `pointer-optimization` | Pointer setting optimization | §カーネル予算 / ポインタ最適化(doc) |
| [169128](https://forums.atariage.com/topic/169128) | `screen-resolution` | What is the Atari 2600 screen resolution? | §カーネル予算 / ライン数設計(doc) |

## Bitmap — ビットマップ・高度カート
| topic | technique | thread | feeds |
|---|---|---|---|
| [119592](https://forums.atariage.com/topic/119592) | `andrews-full-colour-bitmap-mode` | Andrew's Full-Colour Bitmap Mode | §カーネル予算 / bitmap48, DPC |
| [224683](https://forums.atariage.com/topic/224683) | `bang-superchip-demo` | Introducing new 30k/Superchip demo: Bang! | §カーネル予算 / Superchip(高度カートG1) |
| [181816](https://forums.atariage.com/topic/181816) | `bigger-bitmaps-with-dpc` | Bigger Bitmaps with DPC+ | §カーネル予算 / DPC(高度カートG1) |
| [168603](https://forums.atariage.com/topic/168603) | `bitmap-minikernel` | Bitmap Minikernel | §カーネル予算 / bitmap48 |
| [163495](https://forums.atariage.com/topic/163495) | `harmony-dpc-programming` | Harmony DPC+ programming | §カーネル予算 / DPC(高度カートG1) |

## 3D/Vector — 3D・レイキャスト・ベクタ（技候補⑫）
| topic | technique | thread | feeds |
|---|---|---|---|
| [120723](https://forums.atariage.com/topic/120723) | `high-resolution-vector-engine` | High resolution vector engine — ROM デルタテーブルによる固定小数点モーション | ⑫ raycasting/vector(technique候補) |
| [187159](https://forums.atariage.com/topic/187159) | `mode-7-style-graphics` | Mode 7 風グラフィックス | ⑫ mode7(technique候補) |
| [191764](https://forums.atariage.com/topic/191764) | `plotcube` | plotcube | ⑫ vector(technique候補) |
| [328290](https://forums.atariage.com/topic/328290) | `puedo-vector-gfx` | "puedo vector gfx" | ⑫ 擬似ベクタ(technique候補) |
| [337638](https://forums.atariage.com/topic/337638) | `rampart-3d-kernel-test` | Rampart | ⑫ 3Dカーネル(technique候補) |
| [229083](https://forums.atariage.com/topic/229083) | `ray-casting-engine-demo` | Ray Casting Engine Demo | ⑫a raycasting★(Joe's demo・必読) |
| [279712](https://forums.atariage.com/topic/279712) | `raycasting-bus-stuffing` | Raycasting with bus stuffing DEMO | ⑫ raycasting(bus stuffing依存=スコープ外) |
| [338226](https://forums.atariage.com/topic/338226) | `refhraktor` | Refhraktor | ⑫ vector(DaveC作) |
| [321894](https://forums.atariage.com/topic/321894) | `runes-of-moria-3d-first-person` | Runes of Moria — vanilla 4K の一人称3D | ⑫a raycasting(DASM公開ソース有) |

## Audio — 音楽・効果音・音声
| topic | technique | thread | feeds |
|---|---|---|---|
| [234209](https://forums.atariage.com/topic/234209) | `doctor-who-berzerk-speech` | Doctor Who Berzerk | techniques/sound / AUDV 4bit PCM(doc) |
| [309689](https://forums.atariage.com/topic/309689) | `software-speech-synthesizer` | Software Speech Synthesizer for the 2600 / SAM2600 | techniques/sound / デジタル音声(doc) |
| [386896](https://forums.atariage.com/topic/386896) | `tiamat-tia-music-tool` | Tiamat | techniques/music-driver(DaveC作ツール) |
| [250014](https://forums.atariage.com/topic/250014) | `tiatracker` | TIATracker | techniques/music-driver / read_audio |
| [330790](https://forums.atariage.com/topic/330790) | `tiatrackerplus` | TIATrackerPlus | techniques/music-driver |

## Tools — 作画/音ツール（参照・著述には非直結）
| topic | technique | thread | feeds |
|---|---|---|---|
| [349056](https://forums.atariage.com/topic/349056) | `2600-screen-editor` | 2600 Screen Editor v0.7 | reference / 画面エディタ(349056) |
| [69851](https://forums.atariage.com/topic/69851) | `2600-sprite-creators` | 2600 Sprite Creators | reference / スプライト作成ツール |
| [122670](https://forums.atariage.com/topic/122670) | `graphic-software` | 122670 — graphic software that could be good for creating/editing atari graphics | reference / 作画ツール地図 |
| [222889](https://forums.atariage.com/topic/222889) | `my-atari-vcs-sprite-editor` | My Atari VCS Sprite Editor | reference / スプライトエディタ |
| [318184](https://forums.atariage.com/topic/318184) | `playerpal-22` | PlayerPal 2.2 | reference / PlayerPal |
| [69521](https://forums.atariage.com/topic/69521) | `playerpal-v20` | PlayerPal v2.0 設計議論 | reference / PlayerPal |
| [373427](https://forums.atariage.com/topic/373427) | `sprite-animation-editor` | Atari 2600 Sprite Animation Editor | reference / アニメエディタ |
| [198295](https://forums.atariage.com/topic/198295) | `tools-for-graphics-and-sound` | 198295 — Are there tools for graphics and sound? | reference / ツール地図 |

## Reference — 逆アセンブル・参照ゲーム
| topic | technique | thread | feeds |
|---|---|---|---|
| [258191](https://forums.atariage.com/topic/258191) | `bus-stuffing-demos` | Bus Stuffing Demos | 高度カートG1 / bus stuffing(スコープ外) |
| [256141](https://forums.atariage.com/topic/256141) | `disassembling-2600-games` | Disassembling 2600 Games? | reference/disassemblies / cmd/dissect |
| [85667](https://forums.atariage.com/topic/85667) | `medieval-mayhem` | Medieval Mayhem | reference / MM(採掘起点・DaveC) |

## Pizza Boy — Pizza Boy / DaveC（ground-truth）
| topic | technique | thread | feeds |
|---|---|---|---|
| [324603](https://forums.atariage.com/topic/324603) | `legendary-spear` | Legendary Spear | reference / DaveC作 |
| [329673](https://forums.atariage.com/topic/329673) | `pizza-boy-atari-ar` | Pizza Boy / Atari AR 開発スレ | reference/pizza-boy/dissection.ja.md(grade-A正本) |
| [328734](https://forums.atariage.com/topic/328734) | `stocking-stuffer-marble-game` | Stocking Stuffer Marble Game | reference / DaveC作 |
