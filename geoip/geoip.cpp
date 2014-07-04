#include <map>
#include <regex>
#include <string>
#include <fstream>
#include <iostream>
using namespace std;


typedef std::map<unsigned int,std::string> GeoIPDBCXX;

inline vector<string> splite(const std::string & line)
{
	std::regex rgx("[\"]");

	std::sregex_token_iterator iter(line.begin(),line.end(),rgx,-1), end;

	std::vector<string> result;

	for (; iter != end; ++ iter)
	{
		std::string token = *iter;

		if(!token.empty() && token != ",")
		{
			result.push_back(token);
		}
	}

	return result;
}

extern "C" void* geoip_load(const char * csv)
{

	std:ifstream stream(csv);

	if(!stream.is_open())
	{
		std::cout << "can't open file :" <<  csv << std::endl;

		return NULL;
	}
	else
	{
		GeoIPDBCXX * db = new GeoIPDBCXX();

		while(!stream.eof())
		{
			std::string line;
			std::getline(stream,line);

			auto tokens = splite(line);
			
			if(tokens.size() != 6)
			{
				std::cout << "skip error line :" << line << std::endl;
			}
			else
			{
				(*db)[(unsigned int)stol(tokens[2])] = tokens[4];

				(*db)[(unsigned int)stol(tokens[3])] = tokens[4];
			}
		}

		return db;
	}

	
}

extern "C" void geoip_close(void* db)
{
	delete (GeoIPDBCXX*)db;
}

extern "C" const char* geoip_search(void* db, unsigned int ip)
{
	auto & map = *(GeoIPDBCXX*)db;

	auto begin = map.lower_bound(ip);

	auto end = map.upper_bound(ip);

	if(begin == map.end() || end == map.end())
	{
		return NULL;
	}

	if(begin->second != end->second)
	{
		return NULL;
	}

	return begin->second.c_str();
}