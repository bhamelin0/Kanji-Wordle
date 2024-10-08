import { React, useId, useState } from "react";
import * as wanakana from 'wanakana';

function VocabTyper({enabled, onSubmit}) {
    const id = useId();
    const [input, setInput] = useState('');
    const [bound, setBound] = useState(false);
    const inputElem = document.getElementById(id);

    if(input && !bound) {
        wanakana.bind(inputElem);
        console.log("bound!");
        setBound(true);
    }

    function onSubmitHandler(input) {
        onSubmit(input);
        setInput('');
    }

    function handleKeyPress(e) {
        if(e.key !== 'Enter') {
            return;
        }

        onSubmit(input);
        setInput('');
    }

    return (
        <span className="Vocab-Typer">
            <input id={id} disabled={!enabled} className="Vocab-Input" placeholder="Write vocabulary!" value={input} onKeyDown={(e) => handleKeyPress(e)} onInput={e => setInput(e.target.value)}></input>
            <button className="Vocab-Input-Button" onClick={() => onSubmitHandler(input)} disabled={!enabled}>Submit</button>
        </span>

    );
}
  
export default VocabTyper;
  