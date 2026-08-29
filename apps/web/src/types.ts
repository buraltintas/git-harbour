export type Coord={x:number;y:number};
export type Day={date:string;weekday:number;contributionCount:number;contributionLevel:number};
export type Ship={kind:string;cells:Coord[];hits?:Coord[]};
export type Shot=Coord&{result:'miss'|'hit'|'sunk';ship?:string};
export type Stats={games:number;wins:number;losses:number;rating:number;shots:number;hits:number;currentStreak:number;longestStreak:number;winRate:number;accuracy:number;averageShotsPerWin:number;rank:string};
export type Game={id:string;status:'battle'|'complete';turn:string;playerBoard:Day[];enemyBoard:Day[];playerFleet:Ship[];playerShots:Shot[];aiShots:Shot[];playerStart:string;winner?:string;ratingDelta?:number;shareId?:string;enemyPeriod?:{start:string;end:string};stats:Stats};
export const fleetSizes:Record<string,number>={Carrier:5,Battleship:4,Cruiser:3,Submarine:3,Destroyer:2};
