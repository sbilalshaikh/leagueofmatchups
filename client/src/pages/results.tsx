import React, { useState, useEffect } from "react";
import { useLocation } from "react-router-dom";
import { Card, CardHeader, CardContent } from "@/components/ui/card";

function parseSources(points: string[]): string[] {
  const sourceRegex = /\[Sources: \[(.*?)\]\]/g;
  const sources: string[][] = [];

  points.forEach(point => {
    const matches = point.matchAll(sourceRegex);
    for (const match of matches) {
      const sourcesArray = match[1].split(', ');
      sources.push(sourcesArray);
    }
  });

  return sources.flat();
}

const cleanPoint = (point: string): string => {
  const index = point.indexOf('[Sources:');
  if (index !== -1) {
    return point.substring(0, index).trim();
  }
  return point.trim();
};

const Results: React.FC = () => {
  const location = useLocation();
  const { champion, opponent, role } = location.state || {};
  const [data, setData] = useState<{ advice: string } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const targetUrl = `https://www.leagueofmatchups.ai/api/matchup?champ=${champion}&opp=${opponent}&role=${role}`;
        
        const response = await fetch(targetUrl);

        if (!response.ok) {
          throw new Error('Network response was not ok');
        }
        const result = await response.json();
        setData(result);
      } catch (error) {
        setError('Failed to fetch data');
        console.error('Error:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [champion, opponent, role]);

  if (loading) return (
  
  
    <div className="flex items-center justify-center h-screen">
      <div className="text-center">
        <img 
          src="https://i.pinimg.com/originals/2c/37/36/2c3736aa705fc91e770fcfe480b05561.gif" 
          alt="Loading animation"
          className="mx-auto mb-4"
        />
        <h2 className="font-vietnam text-2xl font-medium">
          Getting your summary, shouldn't take longer than 25s
        </h2>
      </div>
    </div>

);
  if (error) return <div>Error: {error}</div>;
  if (!data) return <div>No data available</div>;

  const points: string[] = data.advice.split("\n\n").slice(0, -1);
  const sources = parseSources(points);

  return (
    <div className="min-h-screen text-foreground p-8">
      <h1 className="font-vietnam text-7xl pb-7 font-medium text-left">{champion} vs {opponent} {role}</h1>

      <div className="flex flex-row md:flex-row gap-8">
        <Card className="shadow-lg flex-3">
          <CardHeader className="text-wp italic">
            <h2 className="text-2xl font-semibold">✨Advice</h2>
          </CardHeader>
          <CardContent>
            <ul className="list-disc pl-5 space-y-4">
              {points.map((point, index) => (
                <li key={index} className="text-sm">{cleanPoint(point)}</li>
              ))}
            </ul>
          </CardContent>
        </Card>

        <Card className="shadow-lg flex-1">
          <CardHeader className="text-wp italic">
            <h2 className="text-2xl font-semibold">✨Sources</h2>
          </CardHeader>
          <CardContent>
            <ul className="list-disc pl-5 space-y-2">
              {sources.map((source, index) => (
                <li key={index} className="text-sm">
                  <a href={source} target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:underline">
                    {source}
                  </a>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </div>

      <div className="flex justify-center pt-[1%]">
        <button
          className="font-vietnam bg-beautiful-pink rounded-xl text-white px-20 py-3 hover:bg-opacity-90 text-lg"
        >
          back
        </button>
      </div>
    </div>
  );
};

export default Results;