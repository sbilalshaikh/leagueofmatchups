import React from "react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import rivenImage from "../assets/riven.png";
import yasuoImage from "../assets/yasuo.png";
import dravenImage from "../assets/draven.png";
import ezrealImage from "../assets/ezreal.png";
import champions from "../assets/champions.json";
import { Selectable, Combobox } from "@/components/ui/combobox";

const Home: React.FC = () => {
  const champs: Selectable[] = champions;
  const roles: Selectable[] = [
    { value: "top", label: "Top Laner" },
    { value: "jungle", label: "Jungler" },
    { value: "mid", label: "Mid Laner" },
    { value: "adc", label: "ADC" },
    { value: "support", label: "Support" },
  ];

  const [champion, setChampion] = useState("");
  const [opponent, setOpponent] = useState("");
  const [role, setRole] = useState("");
  const navigator = useNavigate();

  const handleSubmit = () => {
    if (!champion || !opponent || !role) {
      alert("Fill in all fields!");
      return;
    }

    if (champion === opponent) {
      alert("Mirror matchups should be obvious!");
      return;
    }

    
    alert(`${champion} ${opponent}`)
    navigator('/results', { state: { champion:champion , opponent:opponent , role:role  } })

  };

  return (
    <div className="h-screen w-screen">
      <div className="grid grid-cols-2 grid-rows-1 h-full w-full">
        <div
          id="info-card"
          className="pl-28 pt-[3vh] ml-[1vw] mr-[3vw] mt-[1.5vh] flex flex-col"
        >
          <h1 id="title" className="font-vietnam text-7xl pb-7 font-medium">
            League of Matchups
          </h1>
          <div id="imgs" className="flex-1 grid grid-cols-2 grid-rows-2 gap-0">
            <img
              id="riven"
              className="w-full h-full object-cover"
              src={rivenImage}
              alt="Riven"
            />
            <img
              id="yasuo"
              className="w-full h-full object-cover"
              src={yasuoImage}
              alt="Yasuo"
            />
            <img
              id="draven"
              className="w-full h-full object-cover"
              src={dravenImage}
              alt="Draven"
            />
            <img
              id="ezreal"
              className="w-full h-full object-cover"
              src={ezrealImage}
              alt="Ezreal"
            />
          </div>
          <p className="font-vietnam text-blue-600 underline mb-2 mt-2 font-medium]">
            how does it work?
          </p>
        </div>

        <div
          id="input-card"
          className="bg-rose-gold rounded-xl mx-[5vw] mt-[3.5vh] mb-9 bg-opacity-35 drop-shadow-lg flex flex-col p-8"
        >
          <div className="flex flex-col space-y-8">
            <div className="flex flex-col space-y-2 sm:py-[2.5%] md:[2.5%] lg:py-[5%]">
              <h2 className="font-vietnam text-2xl font-semibold">
                I'm playing...
              </h2>
              <Combobox
                selections={champs}
                selectionLabel="Champion"
                width="100%"
                height="75px"
                onSelect={setChampion}
              />
            </div>

            <div className="flex flex-col space-y-2 sm:py-[2.5%] md:[3%] lg:py-[5%]">
              <h2 className="font-vietnam text-2xl font-semibold">
                Against...
              </h2>
              <Combobox
                selections={champs}
                selectionLabel="Champion"
                width="100%"
                height="75px"
                onSelect={setOpponent}
              />
            </div>

            <div className="flex flex-col space-y-2 sm:py-[2.5%] md:[3%] lg:py-[5%]">
              <h2 className="font-vietnam text-2xl font-semibold">As a...</h2>
              <Combobox
                selections={roles}
                selectionLabel="Role"
                width="100%"
                height="75px"
                onSelect={setRole}
              />
            </div>

            <div className="flex justify-center pt-[1%]">
              <button
                className="font-vietnam bg-beautiful-pink rounded-xl text-white px-20 py-3 hover:bg-opacity-90 text-lg"
                onMouseDown={handleSubmit}
              >
                go!
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default Home;
