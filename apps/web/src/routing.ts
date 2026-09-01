export function navigateAfterAuth(next:string,setRoute:(route:string)=>void){
  // The callback already replaced the URL. Update React state explicitly
  // because assigning the same hash does not emit a hashchange event.
  setRoute(next);
  location.hash=next;
}
